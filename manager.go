package poolmanager

import (
	"errors"
	"hash/fnv"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// PoolManager adalah struct untuk mengelola pooling objek
// Menyediakan fitur seperti auto-tuning, sharding, caching, dan eviksi
type PoolManager struct {
	pools             sync.Map         // Menyimpan pool berdasarkan tipe objek
	cache             sync.Map         // Menyimpan cache untuk objek yang sering digunakan
	poolConfig        sync.Map         // Menyimpan konfigurasi untuk setiap pool
	instanceFactories sync.Map         // Menyimpan factory function untuk membuat objek baru
	metrics           sync.Map         // Menyimpan metrik penggunaan pool
	itemMetadata      sync.Map         // Metadata untuk setiap item di pool
	autoTuneTicker    *time.Ticker     // Ticker untuk auto-tuning pool
	autoTuneStop      chan struct{}    // Channel untuk menghentikan auto-tuning
	logger            *log.Logger      // Logger untuk mencatat log pool
	monitoringConfig  MonitoringConfig // Konfigurasi monitoring untuk mencatat metrik
	evictionPolicy    EvictionPolicy   // Kebijakan eviksi yang digunakan untuk pool
	shardCounter      int64            // Counter untuk round-robin sharding
}

// NewPoolManager membuat instance PoolManager baru dengan logger default
// Menginisialisasi channel autoTuneStop dan logger
func NewPoolManager() *PoolManager {
	pm := &PoolManager{
		autoTuneStop: make(chan struct{}),
		logger:       log.New(os.Stdout, "POOL_MANAGER: ", log.LstdFlags),
	}
	return pm
}

// SetMonitoringConfig menetapkan konfigurasi monitoring untuk PoolManager
// MonitoringConfig digunakan untuk mengatur bagaimana log dan metrik dicatat
func (pm *PoolManager) SetMonitoringConfig(config MonitoringConfig) {
	pm.monitoringConfig = config
}

// AddPool menambahkan pool baru dengan tipe tertentu dan konfigurasi yang ditentukan
// poolType: tipe pool yang ditambahkan
// factory: fungsi untuk membuat objek baru dalam pool
// config: konfigurasi untuk pool yang ditambahkan
func (pm *PoolManager) AddPool(poolType string, factory func() PoolAble, config PoolConfig) error {
	if _, exists := pm.pools.Load(poolType); exists {
		return NewPoolError(poolType, "add", errors.New(ErrPoolDoesNotExist+poolType))
	}

	var pool interface{}

	if config.ShardingEnabled && config.ShardCount > 1 {
		shardedPools := make([]*sync.Pool, config.ShardCount)
		for i := 0; i < config.ShardCount; i++ {
			shardedPools[i] = &sync.Pool{New: func() interface{} { return factory() }}
		}
		pool = shardedPools
	} else {
		pool = &sync.Pool{New: func() interface{} { return factory() }}
	}

	pm.pools.Store(poolType, pool)
	pm.poolConfig.Store(poolType, config)
	pm.instanceFactories.Store(poolType, factory)

	if config.InitialSize > 0 {
		for i := 0; i < config.InitialSize; i++ {
			instance := factory()
			if config.ShardingEnabled && config.ShardCount > 1 {
				shardedPools, ok := pool.([]*sync.Pool)
				if !ok {
					return NewPoolError(poolType, "add", errors.New(ErrInvalidShardedPoolType))
				}
				shardIndex := rand.Intn(config.ShardCount)
				shardedPools[shardIndex].Put(instance)
			} else {
				nonShardedPool, ok := pool.(*sync.Pool)
				if !ok {
					return NewPoolError(poolType, "add", errors.New(ErrInvalidNonShardedPoolType))
				}
				nonShardedPool.Put(instance)
			}
		}
	}
	pm.initMetrics(poolType)
	return nil
}

// AcquireInstance mengambil instance dari pool dengan tipe tertentu
// poolType: tipe pool tempat mengambil instance
// Mengembalikan objek PoolAble dan error jika terjadi kesalahan
func (pm *PoolManager) AcquireInstance(poolType string) (PoolAble, error) {
	pool, ok := pm.pools.Load(poolType)
	if !ok {
		err := NewPoolError(poolType, "acquire", errors.New(ErrPoolDoesNotExist+poolType))
		pm.handleError(poolType, err)
		return nil, err
	}

	configVal, _ := pm.poolConfig.Load(poolType)
	conf := configVal.(PoolConfig)

	instance, err := pm.getInstanceFromPool(poolType, pool, conf)
	if err != nil {
		pm.handleError(poolType, err)
		return nil, err
	}

	if instance == nil {
		factoryVal, _ := pm.instanceFactories.Load(poolType)
		factory := factoryVal.(func() PoolAble)
		instance = factory()
	}

	if poolAbleInstance, ok := instance.(PoolAble); ok {
		pm.recordMetric(poolType, "get")
		if conf.EnableCaching {
			pm.cache.Store(poolType, poolAbleInstance)
		}
		if conf.OnGet != nil {
			conf.OnGet(poolType)
		}
		return poolAbleInstance, nil
	}

	err = NewPoolError(poolType, "acquire", errors.New("failed to cast instance to PoolAble"))
	pm.handleError(poolType, err)
	return nil, err
}

// getInstanceFromPool mengambil instance dari pool, dengan dukungan untuk sharding
// poolType: tipe pool tempat mengambil instance
// pool: referensi ke pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// Mengembalikan instance dan error jika terjadi kesalahan
func (pm *PoolManager) getInstanceFromPool(poolType string, pool interface{}, conf PoolConfig) (interface{}, error) {
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		shardedPools, ok := pool.([]*sync.Pool)
		if !ok {
			return nil, NewPoolError(poolType, "get", errors.New(ErrInvalidShardedPoolType))
		}
		shardIndex := pm.getShardIndex(poolType, conf, time.Now().String())
		return shardedPools[shardIndex].Get(), nil
	} else {
		nonShardedPool, ok := pool.(*sync.Pool)
		if !ok {
			return nil, NewPoolError(poolType, "get", errors.New(ErrInvalidNonShardedPoolType))
		}
		return nonShardedPool.Get(), nil
	}
}

// ReleaseInstance mengembalikan instance ke pool dengan tipe tertentu
// poolType: tipe pool tempat mengembalikan instance
// instance: objek yang akan dikembalikan ke pool
func (pm *PoolManager) ReleaseInstance(poolType string, instance PoolAble) error {
	if instance == nil {
		return errors.New("cannot put nil instance into pool")
	}

	poolVal, ok := pm.pools.Load(poolType)
	if !ok {
		return errors.New(ErrPoolDoesNotExist + poolType)
	}

	configVal, _ := pm.poolConfig.Load(poolType)
	conf := configVal.(PoolConfig)

	instance.Reset()

	err := pm.putInstanceToPool(poolType, poolVal, conf, instance)
	if err != nil {
		pm.handleError(poolType, err)
		return err
	}

	pm.recordMetric(poolType, "put")
	if conf.OnPut != nil {
		conf.OnPut(poolType)
	}

	return nil
}

// putInstanceToPool mengembalikan instance ke pool dengan dukungan sharding
// poolType: tipe pool tempat mengembalikan instance
// pool: referensi ke pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// instance: objek yang akan dikembalikan ke pool
func (pm *PoolManager) putInstanceToPool(poolType string, pool interface{}, conf PoolConfig, instance interface{}) error {
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		shardedPools, ok := pool.([]*sync.Pool)
		if !ok {
			return NewPoolError(poolType, "put", errors.New(ErrInvalidShardedPoolType))
		}
		shardIndex := pm.getShardIndex(poolType, conf, time.Now().String())
		shardedPools[shardIndex].Put(instance)
	} else {
		nonShardedPool, ok := pool.(*sync.Pool)
		if !ok {
			return NewPoolError(poolType, "put", errors.New(ErrInvalidNonShardedPoolType))
		}
		nonShardedPool.Put(instance)
	}
	return nil
}

// getShardIndex menghitung indeks shard berdasarkan strategi sharding yang ditentukan
// poolType: tipe pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// key: kunci yang digunakan untuk menghitung indeks shard
func (pm *PoolManager) getShardIndex(poolType string, conf PoolConfig, key string) int {
	switch conf.ShardStrategy {
	case "round-robin":
		return int(atomic.AddInt64(&pm.shardCounter, 1) % int64(conf.ShardCount))
	case "random":
		return rand.Intn(conf.ShardCount)
	case "hash":
		fallthrough
	default:
		return int(hashString(key) % uint32(conf.ShardCount))
	}
}

// hashString menghitung nilai hash dari string menggunakan algoritma hash FNV-1a
// s: string yang akan di-hash
func hashString(s string) uint32 {
	h := fnv.New32a()
	_, err := h.Write([]byte(s))
	if err != nil {
		// Karena error tidak mungkin terjadi pada penulisan ke hash, kita akan panik jika terjadi
		panic("unexpected error while writing to hash: " + err.Error())
	}
	return h.Sum32()
}

// RemovePool menghapus pool tertentu berdasarkan tipe
func (pm *PoolManager) RemovePool(poolType string) error {
	// Hapus pool yang terkait dengan tipe yang diberikan
	pm.pools.Delete(poolType)
	// Hapus konfigurasi pool
	pm.poolConfig.Delete(poolType)
	// Hapus factory instance yang terkait
	pm.instanceFactories.Delete(poolType)
	// Hapus metrik yang terkait dengan pool tersebut
	pm.metrics.Delete(poolType)
	// Hapus cache yang terkait
	pm.cache.Delete(poolType)
	// Hapus metadata item
	pm.itemMetadata.Delete(poolType)

	return nil
}
