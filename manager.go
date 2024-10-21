package poolmanager

import (
	"errors"
	"hash/fnv"
	"log"
	"math/rand"
	"os"
	"sync"
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
	// Ambil konfigurasi pool
	configVal, _ := pm.poolConfig.Load(poolType)
	conf, ok := configVal.(PoolConfig)
	if !ok {
		err := NewPoolError(poolType, "acquire", errors.New(ErrInvalidPoolConfigType))
		pm.handleError(poolType, err)
		return nil, err
	}

	// Coba mengambil dari cache terlebih dahulu jika caching diaktifkan
	if conf.EnableCaching {
		if cachedInstance, found := pm.cache.Load(poolType); found {
			if poolAbleInstance, ok := cachedInstance.(PoolAble); ok {
				// Perbarui metadata saat instance diambil dari cache
				pm.safelyUpdateMetadata(poolType, func(metadata *PoolItemMetadata) {
					metadata.LastUsed = time.Now()
					metadata.Frequency++
					metadata.Status = "Active"
				})

				pm.recordMetric(poolType, "cache_hit")
				if conf.OnGet != nil {
					conf.OnGet(poolType)
				}
				return poolAbleInstance, nil
			}
		}
	}

	// Jika tidak ada di cache, lanjutkan dengan pengambilan dari pool
	pool, ok := pm.pools.Load(poolType)
	if !ok {
		err := NewPoolError(poolType, "acquire", errors.New(ErrPoolDoesNotExist+poolType))
		pm.handleError(poolType, err)
		return nil, err
	}

	// Ambil instance dari pool, dengan dukungan untuk sharding jika diaktifkan
	instance, err := pm.getInstanceFromPool(poolType, pool, conf)
	if err != nil {
		pm.handleError(poolType, err)
		return nil, err
	}

	// Jika instance tidak ada di pool, buat instance baru menggunakan factory
	if instance == nil {
		factoryVal, _ := pm.instanceFactories.Load(poolType)
		factory, ok := factoryVal.(func() PoolAble)
		if !ok {
			err := NewPoolError(poolType, "acquire", errors.New(ErrInvalidFactoryType))
			pm.handleError(poolType, err)
			return nil, err
		}
		instance = factory()
	}

	// Cast instance menjadi PoolAble dan lakukan proses tambahan
	if poolAbleInstance, ok := instance.(PoolAble); ok {
		pm.recordMetric(poolType, "get")

		// Tambahkan instance ke cache jika caching diaktifkan
		if conf.EnableCaching {
			pm.addToCache(poolType, poolAbleInstance)
		}

		// Perbarui metadata saat instance diambil dari pool
		pm.safelyUpdateMetadata(poolType, func(metadata *PoolItemMetadata) {
			metadata.LastUsed = time.Now()
			metadata.Frequency++
			metadata.Status = "Active"
		})

		// Jalankan callback OnGet jika ada
		if conf.OnGet != nil {
			conf.OnGet(poolType)
		}

		return poolAbleInstance, nil
	}

	// Jika cast gagal, kembalikan error
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
	}

	nonShardedPool, ok := pool.(*sync.Pool)
	if !ok {
		return nil, NewPoolError(poolType, "get", errors.New(ErrInvalidNonShardedPoolType))
	}
	return nonShardedPool.Get(), nil
}

// ReleaseInstance mengembalikan instance ke pool dengan tipe tertentu
// poolType: tipe pool tempat mengembalikan instance
// instance: objek yang akan dikembalikan ke pool
func (pm *PoolManager) ReleaseInstance(poolType string, instance PoolAble) error {
	if instance == nil {
		return errors.New("cannot put nil instance into pool")
	}

	// Perbarui metadata saat instance dikembalikan
	pm.safelyUpdateMetadata(poolType, func(metadata *PoolItemMetadata) {
		metadata.LastUsed = time.Now()
		metadata.Status = "Idle"
	})

	poolVal, ok := pm.pools.Load(poolType)
	if !ok {
		return errors.New(ErrPoolDoesNotExist + poolType)
	}

	configVal, _ := pm.poolConfig.Load(poolType)
	conf, ok := configVal.(PoolConfig)
	if !ok {
		return errors.New(ErrInvalidPoolConfigType)
	}

	instance.Reset()

	err := pm.putInstanceToPool(poolType, poolVal, conf, instance)
	if err != nil {
		pm.handleError(poolType, err)
		return err
	}

	pm.recordMetric(poolType, "put")

	// Update cache jika caching diaktifkan
	if conf.EnableCaching {
		pm.addToCache(poolType, instance)
	}

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
	if conf.ShardStrategy != nil {
		return conf.ShardStrategy.GetShardIndex(poolType, conf.ShardCount, key)
	}
	// Default fallback jika ShardStrategy tidak diatur
	return rand.Intn(conf.ShardCount)
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

// GetPoolSize mengembalikan ukuran pool saat ini
func (pm *PoolManager) GetPoolSize(poolType string) int {
	return pm.getPoolCurrentSize(poolType)
}

// GetShardSize mengembalikan ukuran shard tertentu
func (pm *PoolManager) GetShardSize(poolType string, shardIndex int) int {
	return pm.getShardCurrentSize(poolType, shardIndex)
}

func (pm *PoolManager) StartAutoTuning() {
	if pm.autoTuneTicker == nil {
		pm.autoTuneTicker = time.NewTicker(time.Minute) // Set interval auto-tuning
		go func() {
			for {
				select {
				case <-pm.autoTuneTicker.C:
					pm.autoTunePoolSize()
				case <-pm.autoTuneStop:
					pm.autoTuneTicker.Stop()
					return
				}
			}
		}()
	}
}

// StopAutoTuning menghentikan proses auto-tuning pada PoolManager
func (pm *PoolManager) StopAutoTuning() {
	if pm.autoTuneTicker != nil {
		// Kirim sinyal untuk menghentikan auto-tuning
		pm.autoTuneStop <- struct{}{}
		// Hentikan ticker dan tutup channel autoTuneStop
		close(pm.autoTuneStop)
		pm.autoTuneTicker.Stop()
		pm.autoTuneTicker = nil
		pm.autoTuneStop = make(chan struct{}) // Inisialisasi kembali untuk penggunaan di masa mendatang
		pm.logger.Println("Auto-tuning stopped")
	} else {
		pm.logger.Println("Auto-tuning is not running")
	}
}

func (pm *PoolManager) autoTunePoolSize() {
	pm.pools.Range(func(key, value interface{}) bool {
		poolType := key.(string)
		pm.safelyUpdateMetadata(poolType, func(metadata *PoolItemMetadata) {
			metadata.Status = "Tuning"
		})

		configVal, _ := pm.poolConfig.Load(poolType)
		conf, ok := configVal.(PoolConfig)
		if !ok {
			return true
		}
		if conf.AutoTune {
			// Logika untuk menambah atau mengurangi ukuran pool
			currentSize := int32(pm.getCurrentUsage(poolType)) // Pastikan currentSize bertipe int32
			if currentSize > int32(conf.MaxSize) {
				// Kurangi ukuran pool
				pm.ResizePool(poolType, conf.MaxSize)
			} else if currentSize < int32(conf.InitialSize) {
				// Tambah ukuran pool
				pm.ResizePool(poolType, conf.InitialSize)
			}
		}
		return true
	})
}

func (pm *PoolManager) ResizePool(poolType string, newSize int) {
	// Ambil konfigurasi pool saat ini
	poolVal, ok := pm.pools.Load(poolType)
	if !ok {
		pm.logger.Printf("Pool %s does not exist, cannot resize", poolType)
		return
	}

	configVal, _ := pm.poolConfig.Load(poolType)
	conf, ok := configVal.(PoolConfig)
	if !ok {
		pm.logger.Printf("Invalid pool configuration for %s", poolType)
		return
	}

	// Cek apakah sharding diaktifkan
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		// Mengubah ukuran sharded pool
		shardedPools, ok := poolVal.([]*sync.Pool)
		if !ok {
			pm.logger.Printf("Invalid sharded pool type for %s", poolType)
			return
		}

		for i := 0; i < len(shardedPools); i++ {
			currentSize := pm.getShardCurrentSize(poolType, i)
			if currentSize < newSize {
				// Tambah objek ke shard untuk mencapai ukuran baru
				for j := currentSize; j < newSize; j++ {
					instance := pm.createInstance(poolType)
					shardedPools[i].Put(instance)
				}
			} else if currentSize > newSize {
				// Kurangi objek dari shard untuk mencapai ukuran baru
				for j := currentSize; j > newSize; j-- {
					shardedPools[i].Get() // Ambil dan buang objek
				}
			}
		}
	} else {
		// Mengubah ukuran non-sharded pool
		nonShardedPool, ok := poolVal.(*sync.Pool)
		if !ok {
			pm.logger.Printf("Invalid non-sharded pool type for %s", poolType)
			return
		}

		currentSize := pm.getPoolCurrentSize(poolType)
		if currentSize < newSize {
			// Tambah objek ke pool untuk mencapai ukuran baru
			for i := currentSize; i < newSize; i++ {
				instance := pm.createInstance(poolType)
				nonShardedPool.Put(instance)
			}
		} else if currentSize > newSize {
			// Kurangi objek dari pool untuk mencapai ukuran baru
			for i := currentSize; i > newSize; i-- {
				nonShardedPool.Get() // Ambil dan buang objek
			}
		}
	}

	pm.logger.Printf("Resizing pool %s to new size: %d", poolType, newSize)
}

func (pm *PoolManager) createInstance(poolType string) PoolAble {
	factoryVal, _ := pm.instanceFactories.Load(poolType)
	factory, ok := factoryVal.(func() PoolAble)
	if !ok {
		pm.logger.Printf("Invalid factory for pool type %s", poolType)
		return nil
	}
	return factory()
}

func (pm *PoolManager) getPoolCurrentSize(poolType string) int {
	size := 0
	// Hitung jumlah objek di pool
	pm.cache.Range(func(key, value interface{}) bool {
		if key.(string) == poolType {
			size++
		}
		return true
	})
	return size
}

func (pm *PoolManager) getShardCurrentSize(poolType string, shardIndex int) int {
	// Ambil pool dan konfigurasinya
	poolVal, ok := pm.pools.Load(poolType)
	if !ok {
		pm.logger.Printf("Pool %s does not exist", poolType)
		return 0
	}

	configVal, _ := pm.poolConfig.Load(poolType)
	conf, ok := configVal.(PoolConfig)
	if !ok || !conf.ShardingEnabled || conf.ShardCount <= shardIndex {
		pm.logger.Printf("Invalid configuration for shard %d of pool %s", shardIndex, poolType)
		return 0
	}

	// Ambil sharded pool
	shardedPools, ok := poolVal.([]*sync.Pool)
	if !ok || len(shardedPools) <= shardIndex {
		pm.logger.Printf("Invalid sharded pool type for %s", poolType)
		return 0
	}

	// Dapatkan ukuran cache yang sesuai dengan shardIndex
	size := 0
	pm.cache.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok && keyStr == poolType {
			if shardVal, ok := value.(int); ok && shardVal == shardIndex {
				size++
			}
		}
		return true
	})
	return size
}
