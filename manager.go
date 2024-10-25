package poolmanager

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// PoolManager adalah struct untuk mengelola pooling objek
// Menyediakan fitur seperti auto-tuning, sharding, caching, dan eviksi
type PoolManager struct {
	pools             sync.Map         // Menyimpan pool berdasarkan tipe objek
	poolConfig        sync.Map         // Menyimpan konfigurasi untuk setiap pool
	instanceFactories sync.Map         // Menyimpan factory function untuk membuat objek baru
	metrics           sync.Map         // Menyimpan metrik penggunaan pool
	itemMetadata      sync.Map         // Metadata untuk setiap item di pool
	autoTuneTicker    *time.Ticker     // Ticker untuk auto-tuning pool
	autoTuneStop      chan struct{}    // Channel untuk menghentikan auto-tuning
	logger            *log.Logger      // Logger untuk mencatat log pool
	monitoringConfig  MonitoringConfig // Konfigurasi monitoring untuk mencatat metrik
	evictionPolicy    EvictionPolicy   // Kebijakan eviksi yang digunakan untuk pool
	shardingStrategy  ShardingStrategy // Strategi sharding untuk membagi pool
	shardCounter      int64            // Counter untuk round-robin sharding
	cache             sync.Map         // Menyimpan cache untuk objek yang sering digunakan
}

// InitializePool menginisialisasi pool baru dengan konfigurasi yang diberikan.
// poolName: tipe objek pool yang ingin dibuat.
// config: konfigurasi pool yang digunakan.
// factory: fungsi untuk membuat objek baru yang akan dimasukkan ke dalam pool.
// InitializePool menginisialisasi pool baru dengan konfigurasi yang diberikan.
func (pm *PoolManager) InitializePool(poolName string, config PoolConfiguration, factory func() interface{}) error {
	// Periksa apakah pool sudah ada
	if _, exists := pm.pools.Load(poolName); exists {
		return errors.New("pool already exists: " + poolName)
	}

	// Membuat sync.Pool baru
	newPool := &sync.Pool{
		New: factory,
	}

	// Simpan konfigurasi dan pool ke dalam map
	pm.pools.Store(poolName, newPool)
	pm.poolConfig.Store(poolName, config)
	pm.instanceFactories.Store(poolName, factory)

	// Log inisialisasi pool
	pm.logger.Println("Initializing pool:", poolName)
	pm.logger.Println("Pool configuration:", config)

	// Inisialisasi auto-tuning jika diaktifkan dan intervalnya positif
	if config.AutoTune && config.AutoTuneInterval > 0 {
		pm.autoTuneTicker = time.NewTicker(config.AutoTuneInterval)
		go pm.autoTune(poolName, config)
	} else if config.AutoTune {
		// Log jika AutoTuneInterval tidak valid
		pm.logger.Println("Invalid AutoTuneInterval, auto-tuning not started for pool:", poolName)
	}

	// Mengisi pool dengan objek berdasarkan initialSize dari konfigurasi
	for i := 0; i < config.InitialSize; i++ {
		newPool.Put(factory())
	}

	// Mengatur sharding jika diaktifkan
	if config.ShardingEnabled {
		pm.shardingStrategy = config.ShardStrategy
		pm.shardCounter = int64(config.ShardCount)
		pm.logger.Println("Sharding enabled for pool:", poolName, "Shard count:", config.ShardCount)
	}

	// Mengatur kebijakan eviction
	pm.evictionPolicy = config.Eviction
	if config.TTL > 0 {
		go pm.runEviction(poolName, config.EvictionInterval)
		pm.logger.Println("Eviction policy set for pool:", poolName, "TTL:", config.TTL)
	}

	return nil
}

// NewPoolManager membuat instance PoolManager baru dengan logger default
// Menginisialisasi channel autoTuneStop dan logger
func NewPoolManager(config PoolConfiguration) *PoolManager {
	// Membuat PoolManager baru dengan konfigurasi yang diberikan
	pm := &PoolManager{
		autoTuneStop:     make(chan struct{}),                                 // Channel untuk menghentikan auto-tuning
		logger:           log.New(os.Stdout, "POOL_MANAGER: ", log.LstdFlags), // Logger default
		shardingStrategy: config.ShardStrategy,                                // Gunakan strategi sharding dari konfigurasi
		evictionPolicy:   config.Eviction,                                     // Kebijakan eviksi dari konfigurasi
		monitoringConfig: MonitoringConfig{},                                  // Konfigurasi monitoring default
	}

	// Inisialisasi peta (sync.Map) lainnya untuk memastikan siap digunakan
	pm.pools = sync.Map{}
	pm.poolConfig = sync.Map{}
	pm.instanceFactories = sync.Map{}
	pm.metrics = sync.Map{}
	pm.itemMetadata = sync.Map{}
	pm.cache = sync.Map{}

	// Jika AutoTune diaktifkan, mulai ticker untuk auto-tuning
	if config.AutoTune && config.AutoTuneInterval > 0 {
		pm.autoTuneTicker = time.NewTicker(config.AutoTuneInterval)
		go pm.autoTune(config.Name, config)
	}

	// Jika TTL diatur, jalankan kebijakan eviksi
	if config.TTL > 0 {
		go pm.runEviction(config.Name, config.EvictionInterval)
	}

	return pm
}

// SetMonitoringConfig menetapkan konfigurasi monitoring untuk PoolManager
// MonitoringConfig digunakan untuk mengatur bagaimana log dan metrik dicatat
func (pm *PoolManager) SetMonitoringConfig(config MonitoringConfig) {
	pm.monitoringConfig = config
}

// AddPool menambahkan pool baru dengan tipe tertentu dan konfigurasi yang ditentukan
// poolName: tipe pool yang ditambahkan
// factory: fungsi untuk membuat objek baru dalam pool
// config: konfigurasi untuk pool yang ditambahkan
func (pm *PoolManager) AddPool(poolName string, factory func() PoolAble, config PoolConfiguration) error {
	if _, exists := pm.pools.Load(poolName); exists {
		return NewPoolError(poolName, "add", errors.New(ErrPoolDoesNotExist+poolName))
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

	pm.pools.Store(poolName, pool)
	pm.poolConfig.Store(poolName, config)
	pm.instanceFactories.Store(poolName, factory)

	if config.InitialSize > 0 {
		for i := 0; i < config.InitialSize; i++ {
			instance := factory()

			// Panggil callback OnCreate jika ada
			if config.OnCreate != nil {
				config.OnCreate(poolName, instance)
			}

			if config.ShardingEnabled && config.ShardCount > 1 {
				shardedPools, ok := pool.([]*sync.Pool)
				if !ok {
					return NewPoolError(poolName, "add", errors.New(ErrInvalidShardedPoolName))
				}

				// Menggunakan generator nomor acak yang aman
				shardIndex, err := rand.Int(rand.Reader, big.NewInt(int64(config.ShardCount)))
				if err != nil {
					// Tangani kesalahan jika generator nomor acak gagal
					pm.logger.Printf("Failed to generate secure random number for sharding: %v", err)
					shardIndex = big.NewInt(0) // Fallback ke indeks shard 0 jika terjadi kesalahan
				}

				shardedPools[int(shardIndex.Int64())].Put(instance)
			} else {
				nonShardedPool, ok := pool.(*sync.Pool)
				if !ok {
					return NewPoolError(poolName, "add", errors.New(ErrInvalidNonShardedPoolName))
				}
				nonShardedPool.Put(instance)
			}
		}
	}
	pm.initMetrics(poolName)
	return nil
}

// AcquireInstance mengambil instance dari pool dengan tipe tertentu
// poolName: tipe pool tempat mengambil instance
// Mengembalikan objek PoolAble dan error jika terjadi kesalahan
func (pm *PoolManager) AcquireInstance(poolName string) (PoolAble, error) {
	// Ambil konfigurasi pool
	conf, err := pm.getPoolConfiguration(poolName)
	if err != nil {
		pm.handleError(poolName, err)
		return nil, err
	}

	// Coba mengambil dari cache terlebih dahulu jika caching diaktifkan
	if conf.EnableCaching {
		if cachedInstance, found := pm.cache.Load(poolName); found {
			if poolAbleInstance, ok := cachedInstance.(PoolAble); ok {
				// Perbarui metadata saat instance diambil dari cache
				pm.updateMetadata(poolName, "Active")
				pm.recordMetric(poolName, "cache_hit")
				pm.triggerCallback(conf.OnGet, poolName)
				return poolAbleInstance, nil
			}
		}
	}

	// Jika tidak ada di cache, lanjutkan dengan pengambilan dari pool
	pool, ok := pm.pools.Load(poolName)
	if !ok {
		err := errors.New("pool does not exist: " + poolName)
		pm.handleError(poolName, err)
		return nil, err
	}

	// Ambil instance dari pool, dengan dukungan untuk sharding jika diaktifkan
	instance, err := pm.getInstanceFromPool(poolName, pool, conf)
	if err != nil {
		pm.handleError(poolName, err)
		return nil, err
	}

	// Jika instance tidak ada di pool, buat instance baru menggunakan factory
	if instance == nil {
		factoryVal, _ := pm.instanceFactories.Load(poolName)
		factory, ok := factoryVal.(func() PoolAble)
		if !ok {
			err := errors.New("invalid factory for pool: " + poolName)
			pm.handleError(poolName, err)
			return nil, err
		}
		instance = factory()
	}

	// Cast instance menjadi PoolAble dan lakukan proses tambahan
	if poolAbleInstance, ok := instance.(PoolAble); ok {
		pm.recordMetric(poolName, "get")

		// Tambahkan instance ke cache jika caching diaktifkan
		if conf.EnableCaching {
			pm.addToCache(poolName, poolAbleInstance)
		}

		// Perbarui metadata saat instance diambil dari pool
		pm.updateMetadata(poolName, "Active")
		pm.triggerCallback(conf.OnGet, poolName)

		return poolAbleInstance, nil
	}

	// Jika cast gagal, kembalikan error
	err = errors.New("failed to cast instance to PoolAble")
	pm.handleError(poolName, err)
	return nil, err
}

// getInstanceFromPool mengambil instance dari pool, dengan dukungan untuk sharding
// poolName: tipe pool tempat mengambil instance
// pool: referensi ke pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// Mengembalikan instance dan error jika terjadi kesalahan
func (pm *PoolManager) getInstanceFromPool(poolName string, pool interface{}, conf PoolConfiguration) (interface{}, error) {
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		shardedPools, ok := pool.([]*sync.Pool)
		if !ok {
			return nil, NewPoolError(poolName, "get", errors.New(ErrInvalidShardedPoolName))
		}

		// Pastikan jumlah shard sesuai dengan konfigurasi
		if len(shardedPools) != conf.ShardCount {
			return nil, NewPoolError(poolName, "get", errors.New("shard count mismatch with configuration"))
		}

		// Hitung indeks shard
		shardIndex := pm.getShardIndex(poolName, conf, time.Now().String())

		// Pastikan indeks shard dalam batas array
		if shardIndex < 0 || shardIndex >= len(shardedPools) {
			return nil, NewPoolError(poolName, "get", errors.New("shard index out of range"))
		}

		// Ambil instance dari shard yang dipilih
		instance := shardedPools[shardIndex].Get()
		if instance == nil {
			return nil, NewPoolError(poolName, "get", errors.New("no instance available in the selected shard"))
		}
		return instance, nil
	}

	// Pengambilan dari pool yang tidak menggunakan sharding
	nonShardedPool, ok := pool.(*sync.Pool)
	if !ok {
		return nil, NewPoolError(poolName, "get", errors.New(ErrInvalidNonShardedPoolName))
	}

	// Ambil instance dari pool
	instance := nonShardedPool.Get()
	if instance == nil {
		return nil, NewPoolError(poolName, "get", errors.New("no instance available in the non-sharded pool"))
	}
	return instance, nil
}

// ReleaseInstance mengembalikan instance ke pool dengan tipe tertentu
// poolName: tipe pool tempat mengembalikan instance
// instance: objek yang akan dikembalikan ke pool
func (pm *PoolManager) ReleaseInstance(poolName string, instance PoolAble) error {
	if instance == nil {
		err := errors.New("cannot put nil instance into pool")
		pm.handleError(poolName, err)
		return err
	}

	// Perbarui metadata saat instance dikembalikan
	pm.updateMetadata(poolName, "Idle")

	// Ambil pool dan konfigurasi
	poolVal, ok := pm.pools.Load(poolName)
	if !ok {
		err := errors.New("pool does not exist: " + poolName)
		pm.handleError(poolName, err)
		return err
	}

	conf, err := pm.getPoolConfiguration(poolName)
	if err != nil {
		pm.handleError(poolName, err)
		return err
	}

	// Reset instance sebelum mengembalikan ke pool
	instance.Reset()

	// Panggil callback OnReset jika ada
	pm.triggerCallbackWithInstance(conf.OnReset, poolName, instance)

	// Masukkan instance kembali ke pool
	err = pm.putInstanceToPool(poolName, poolVal, conf, instance)
	if err != nil {
		pm.handleError(poolName, err)
		return err
	}

	pm.recordMetric(poolName, "put")

	// Update cache jika caching diaktifkan
	if conf.EnableCaching {
		pm.addToCache(poolName, instance)
	}

	// Panggil callback OnPut jika ada
	pm.triggerCallback(conf.OnPut, poolName)

	return nil
}

// putInstanceToPool mengembalikan instance ke pool dengan dukungan sharding
// poolName: tipe pool tempat mengembalikan instance
// pool: referensi ke pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// instance: objek yang akan dikembalikan ke pool
func (pm *PoolManager) putInstanceToPool(poolName string, pool interface{}, conf PoolConfiguration, instance interface{}) error {
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		shardedPools, ok := pool.([]*sync.Pool)
		// reset instance
		if !ok {
			return NewPoolError(poolName, "put", errors.New(ErrInvalidShardedPoolName))
		}
		shardIndex := pm.getShardIndex(poolName, conf, time.Now().String())
		shardedPools[shardIndex].Put(instance)
	} else {
		nonShardedPool, ok := pool.(*sync.Pool)
		if !ok {
			return NewPoolError(poolName, "put", errors.New(ErrInvalidNonShardedPoolName))
		}
		nonShardedPool.Put(instance)
	}
	return nil
}

// getShardIndex menghitung indeks shard berdasarkan strategi sharding yang ditentukan
// poolName: tipe pool yang digunakan
// conf: konfigurasi untuk pool yang digunakan
// key: kunci yang digunakan untuk menghitung indeks shard
func (pm *PoolManager) getShardIndex(poolName string, conf PoolConfiguration, key string) int {
	hashValue := hashString(key)
	return int(hashValue) % conf.ShardCount
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
func (pm *PoolManager) RemovePool(poolName string) error {
	// Hapus pool yang terkait dengan tipe yang diberikan
	pm.pools.Delete(poolName)
	// Hapus konfigurasi pool
	pm.poolConfig.Delete(poolName)
	// Hapus factory instance yang terkait
	pm.instanceFactories.Delete(poolName)
	// Hapus metrik yang terkait dengan pool tersebut
	pm.metrics.Delete(poolName)
	// Hapus cache yang terkait
	pm.cache.Delete(poolName)
	// Hapus metadata item
	pm.itemMetadata.Delete(poolName)

	return nil
}

// GetPoolSize mengembalikan ukuran pool saat ini
func (pm *PoolManager) GetPoolSize(poolName string) int {
	return pm.getPoolCurrentSize(poolName)
}

// GetShardSize mengembalikan ukuran shard tertentu
func (pm *PoolManager) GetShardSize(poolName string, shardIndex int) int {
	return pm.getShardCurrentSize(poolName, shardIndex)
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
					if pm.autoTuneTicker != nil {
						pm.autoTuneTicker.Stop() // Pastikan autoTuneTicker dihentikan
						pm.autoTuneTicker = nil
					}
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
		select {
		case pm.autoTuneStop <- struct{}{}:
			// Channel belum tertutup, kirim sinyal
		default:
			// Channel sudah tertutup, abaikan
		}

		// Hentikan ticker dan pastikan `autoTuneTicker` benar-benar dihentikan
		pm.autoTuneTicker.Stop()
		pm.autoTuneTicker = nil

		// Tutup channel autoTuneStop dengan aman
		defer func() {
			if r := recover(); r == nil {
				close(pm.autoTuneStop)
			}
		}()

		// Inisialisasi kembali untuk penggunaan di masa mendatang
		pm.autoTuneStop = make(chan struct{})
		pm.logger.Println("Auto-tuning stopped")
	} else {
		pm.logger.Println("Auto-tuning is not running")
	}
}

// getCurrentPoolSize menghitung ukuran pool saat ini berdasarkan poolName dan nilai pool.
func (pm *PoolManager) getCurrentPoolSize(poolName string, value interface{}) int {
	if shardedPools, isSharded := value.([]*sync.Pool); isSharded {
		// Jika pool adalah array dari sync.Pool (sharded), hitung total ukuran dari semua shard
		totalSize := 0
		for shardIndex := range shardedPools {
			totalSize += pm.getShardSize(poolName, shardIndex)
		}
		return totalSize
	} else if _, isNonSharded := value.(*sync.Pool); isNonSharded {
		// Jika pool adalah sync.Pool biasa (non-sharded), hitung ukuran pool
		return pm.getNonShardedPoolSize(poolName)
	}
	// Jika tipe tidak diketahui, gunakan metode default
	return int(pm.getCurrentUsage(poolName))
}

func (pm *PoolManager) ResizePool(poolName string, newSize int) {
	// Ambil konfigurasi pool saat ini
	poolVal, ok := pm.pools.Load(poolName)
	if !ok {
		pm.logger.Printf("Pool %s does not exist, cannot resize", poolName)
		return
	}

	configVal, _ := pm.poolConfig.Load(poolName)
	conf, ok := configVal.(PoolConfiguration)
	if !ok {
		pm.logger.Printf("Invalid pool configuration for %s", poolName)
		return
	}

	// Cek apakah sharding diaktifkan
	if conf.ShardingEnabled && conf.ShardCount > 1 {
		// Mengubah ukuran sharded pool
		shardedPools, ok := poolVal.([]*sync.Pool)
		if !ok {
			pm.logger.Printf("Invalid sharded pool type for %s", poolName)
			return
		}

		for i := 0; i < len(shardedPools); i++ {
			currentSize := pm.getShardCurrentSize(poolName, i)
			if currentSize < newSize {
				// Tambah objek ke shard untuk mencapai ukuran baru
				for j := currentSize; j < newSize; j++ {
					instance := pm.createInstance(poolName)
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
			pm.logger.Printf("Invalid non-sharded pool type for %s", poolName)
			return
		}

		currentSize := pm.getPoolCurrentSize(poolName)
		if currentSize < newSize {
			// Tambah objek ke pool untuk mencapai ukuran baru
			for i := currentSize; i < newSize; i++ {
				instance := pm.createInstance(poolName)
				nonShardedPool.Put(instance)
			}
		} else if currentSize > newSize {
			// Kurangi objek dari pool untuk mencapai ukuran baru
			for i := currentSize; i > newSize; i-- {
				nonShardedPool.Get() // Ambil dan buang objek
			}
		}
	}

	pm.logger.Printf("Resizing pool %s to new size: %d", poolName, newSize)
}

func (pm *PoolManager) createInstance(poolName string) PoolAble {
	factoryVal, _ := pm.instanceFactories.Load(poolName)
	factory, ok := factoryVal.(func() PoolAble)
	if !ok {
		pm.logger.Printf("Invalid factory for pool type %s", poolName)
		return nil
	}
	return factory()
}

func (pm *PoolManager) getPoolCurrentSize(poolName string) int {
	size := 0
	// Hitung jumlah objek di pool
	pm.cache.Range(func(key, value interface{}) bool {
		if key.(string) == poolName {
			size++
		}
		return true
	})
	return size
}

func (pm *PoolManager) getShardCurrentSize(poolName string, shardIndex int) int {
	// Ambil pool dan konfigurasinya
	poolVal, ok := pm.pools.Load(poolName)
	if !ok {
		pm.logger.Printf("Pool %s does not exist", poolName)
		return 0
	}

	configVal, _ := pm.poolConfig.Load(poolName)
	conf, ok := configVal.(PoolConfiguration)
	if !ok || !conf.ShardingEnabled || conf.ShardCount <= shardIndex {
		pm.logger.Printf("Invalid configuration for shard %d of pool %s", shardIndex, poolName)
		return 0
	}

	// Ambil sharded pool
	shardedPools, ok := poolVal.([]*sync.Pool)
	if !ok || len(shardedPools) <= shardIndex {
		pm.logger.Printf("Invalid sharded pool type for %s", poolName)
		return 0
	}

	// Dapatkan ukuran cache yang sesuai dengan shardIndex
	size := 0
	pm.cache.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok && keyStr == poolName {
			if shardVal, ok := value.(int); ok && shardVal == shardIndex {
				size++
			}
		}
		return true
	})
	return size
}

// Reset mengatur ulang objek dalam pool
func (pm *PoolManager) Reset(poolName string) error {
	if _, ok := pm.pools.Load(poolName); ok {
		pm.pools.Delete(poolName)
		return nil
	}
	return errors.New("pool does not exist: " + poolName)
}

// Clear membersihkan semua pool
func (pm *PoolManager) Clear() {
	pm.pools.Range(func(key, value interface{}) bool {
		pm.pools.Delete(key)
		return true
	})
}

// AddShard menambahkan shard baru ke PoolManager
func (pm *PoolManager) AddShard() {
	atomic.AddInt64(&pm.shardCounter, 1)
	pm.logMessage(InfoLevel, "Shard added. Total shards: "+fmt.Sprint(pm.shardCounter))
}

// RemoveShard menghapus shard jika jumlah shard lebih dari 0
func (pm *PoolManager) RemoveShard() error {
	if pm.shardCounter > 0 {
		atomic.AddInt64(&pm.shardCounter, -1)
		pm.logMessage(InfoLevel, "Shard removed. Total shards: "+fmt.Sprint(pm.shardCounter))
		return nil
	}
	return errors.New("no shard available to remove")
}

// HandleError mengatur bagaimana error diproses
func (pm *PoolManager) HandleError(err error) {
	if pm.logger != nil {
		pm.logger.Println("Error:", err)
	} else {
		log.Println("Error:", err)
	}
}

// autoTune menyesuaikan ukuran pool secara otomatis berdasarkan konfigurasi.
func (pm *PoolManager) autoTune(poolName string, config PoolConfiguration) {
	for {
		select {
		case <-pm.autoTuneTicker.C:
			currentSize := pm.GetPoolSize(poolName)
			if currentSize == 0 {
				pm.logger.Println("Auto-tuning skipped, pool is empty:", poolName)
				continue
			}

			newSize := int(float64(currentSize) * config.AutoTuneFactor)
			if newSize > config.MaxSize {
				newSize = config.MaxSize
			} else if newSize < config.MinSize {
				newSize = config.MinSize
			}

			// Hanya ubah ukuran pool jika ada perubahan
			if newSize != currentSize {
				pm.ResizePool(poolName, newSize)
				if config.OnAutoTune != nil {
					config.OnAutoTune(poolName, newSize)
				}
				pm.logger.Printf("Auto-tuned pool %s to new size: %d", poolName, newSize)
			}
		case <-pm.autoTuneStop:
			return
		}
	}
}

// runEviction menjalankan kebijakan eviksi pada interval tertentu.
func (pm *PoolManager) runEviction(poolName string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Jalankan kebijakan eviksi
			if pm.evictionPolicy != nil {
				pm.evictionPolicy.Evict(poolName, pm)
			}
		case <-pm.autoTuneStop:
			// Hentikan eviksi jika auto-tuning dihentikan
			return
		}
	}
}

// evictOldestCacheItem menghapus item cache tertua atau yang paling jarang digunakan
// poolName: tipe pool dari mana item akan dihapus
// Fungsi ini mencari item dengan waktu terakhir digunakan paling lama dan menghapusnya dari cache dan metadata.
func (pm *PoolManager) evictOldestCacheItem(poolName string) {
	// Menggunakan metadata untuk mencari item dengan waktu terakhir digunakan paling lama
	var oldestKey string
	var oldestTime time.Time

	// Iterasi melalui item metadata untuk poolName
	pm.itemMetadata.Range(func(key, value interface{}) bool {
		if itemMeta, ok := value.(*PoolItemMetadata); ok {
			// Pastikan key sesuai dengan poolName
			if k, ok := key.(string); ok && k == poolName {
				if oldestTime.IsZero() || itemMeta.LastUsed.Before(oldestTime) {
					oldestKey = k
					oldestTime = itemMeta.LastUsed
				}
			}
		}
		return true
	})

	// Jika ditemukan item untuk dihapus, hapus dari cache dan metadata
	if oldestKey != "" {
		pm.cache.Delete(oldestKey)
		pm.itemMetadata.Delete(oldestKey)
	}
}

// SetEvictionPolicy mengganti kebijakan eviksi yang digunakan oleh PoolManager
func (pm *PoolManager) SetEvictionPolicy(policy EvictionPolicy) {
	pm.evictionPolicy = policy
}

// ForceEvict secara paksa menghapus objek dari pool berdasarkan kunci
func (pm *PoolManager) ForceEvict(poolName, key string) error {
	// Cek apakah metadata untuk item tersebut ada
	if metadataVal, ok := pm.itemMetadata.Load(key); ok {
		// Pastikan metadata tersebut terkait dengan poolName yang diberikan
		if metadata, ok := metadataVal.(*PoolItemMetadata); ok && metadata.PoolName == poolName {
			// Hapus item dari metadata
			pm.itemMetadata.Delete(key)
			// Hapus item dari cache juga
			pm.cache.Delete(key)

			// Tambahkan log untuk melacak eviksi
			pm.logger.Printf("Force evicted item from pool: %s, Key: %s", poolName, key)
			return nil
		}
	}

	return errors.New("item does not exist in metadata for pool: " + poolName + ", key: " + key)
}

// SetShardingStrategy menetapkan strategi sharding yang akan digunakan oleh PoolManager.
// strategy: strategi sharding yang diimplementasikan oleh pengguna.
func (pm *PoolManager) SetShardingStrategy(strategy ShardingStrategy) {
	pm.shardingStrategy = strategy
	pm.logMessage(InfoLevel, "Sharding strategy set.")
}

// addToCache menambahkan instance ke dalam cache pool
// poolName: tipe pool yang digunakan untuk identifikasi cache
// instance: objek yang akan disimpan dalam cache
// Fungsi ini akan memeriksa konfigurasi pool untuk melihat apakah caching diaktifkan. Jika ukuran cache
// melebihi batas yang ditetapkan, fungsi ini akan menghapus item cache yang paling lama atau jarang digunakan.
func (pm *PoolManager) addToCache(poolName string, instance PoolAble) {
	// Load the pool configuration for the given pool type
	configVal, ok := pm.poolConfig.Load(poolName)
	if !ok {
		// Jika konfigurasi tidak ada, keluar dari fungsi
		return
	}

	// Melakukan type assertion untuk mendapatkan konfigurasi PoolConfiguration
	conf, ok := configVal.(PoolConfiguration)
	if !ok {
		// Jika type assertion gagal, keluar dari fungsi
		return
	}

	// Cek apakah caching diaktifkan
	if conf.EnableCaching {
		cacheSize := pm.getCacheSize(poolName)
		if cacheSize >= conf.CacheMaxSize {
			// Hapus item cache tertua atau LRU jika ukuran cache melebihi batas
			pm.evictOldestCacheItem(poolName)
			// Panggil callback OnDestroy jika ada
			if conf.OnDestroy != nil {
				conf.OnDestroy(poolName, instance)
			}
		}
		// Simpan instance dalam cache
		pm.cache.Store(poolName, instance)
	}
}

// getCacheSize mendapatkan jumlah item dalam cache untuk tipe pool tertentu
// poolName: tipe pool yang digunakan untuk identifikasi cache
// Fungsi ini mengembalikan jumlah objek yang ada dalam cache untuk tipe pool yang diberikan.
func (pm *PoolManager) getCacheSize(poolName string) int {
	size := 0
	pm.cache.Range(func(key, value interface{}) bool {
		if key.(string) == poolName {
			size++
		}
		return true
	})
	return size
}

// handleError memanggil callback OnError pada PoolConfiguration jika error terjadi
// poolName: tipe pool tempat kesalahan terjadi
// err: error yang terjadi selama operasi
// Jika konfigurasi pool memiliki callback OnError, fungsi ini akan memanggil callback tersebut
// dengan parameter poolName dan error yang terjadi.
func (pm *PoolManager) handleError(poolName string, err error) {
	config, _ := pm.poolConfig.Load(poolName)
	if conf, ok := config.(PoolConfiguration); ok && conf.OnError != nil {
		conf.OnError(poolName, err)
	}
}

// logMessage mencatat pesan dengan level log yang ditentukan
func (pm *PoolManager) logMessage(level LogLevel, message string) {
	if level >= pm.monitoringConfig.LogLevel {
		pm.logger.Println(message)
	}
}

func (pm *PoolManager) AddItemMetadata(poolName, key string) {
	metadata := &PoolItemMetadata{
		PoolName:     poolName,
		CreationTime: time.Now(),
		LastUsed:     time.Now(),
		Status:       "Active",
		IsPooled:     true,
	}
	pm.itemMetadata.Store(key, metadata)
}

// UpdateItemMetadata memperbarui metadata item saat diakses
// key: kunci unik yang mengidentifikasi item dalam metadata map
// Fungsi ini memperbarui informasi seperti waktu terakhir digunakan, frekuensi
// penggunaan, dan durasi penggunaan berdasarkan waktu yang telah berlalu sejak
// terakhir kali item digunakan.
func (pm *PoolManager) UpdateItemMetadata(poolName, key string) {
	pm.safelyUpdateMetadata(key, func(metadata *PoolItemMetadata) {
		if metadata.Status == "Evicted" {
			return
		}
		elapsed := time.Since(metadata.LastUsed)
		metadata.UsageDuration += elapsed
		metadata.LastUsed = time.Now()
		metadata.Frequency++
		metadata.Status = "Active"
	})
}

func (pm *PoolManager) ShouldEvictItem(key string, metadata *PoolItemMetadata) bool {
	now := time.Now()
	if metadata.ExpirationTime != nil && now.After(*metadata.ExpirationTime) {
		return true
	}

	if metadata.MaxUsageDuration > 0 && metadata.UsageDuration > metadata.MaxUsageDuration {
		return true
	}

	if metadata.IdleDuration > 0 && now.Sub(metadata.LastUsed) > metadata.IdleDuration {
		return true
	}
	return false
}

func (pm *PoolManager) ResetItemMetadata(key string) {
	pm.safelyUpdateMetadata(key, func(metadata *PoolItemMetadata) {
		metadata.LastUsed = time.Now()
		metadata.Frequency = 0
		metadata.Status = "Idle"
		metadata.LastResetTime = time.Now()
	})
}

// GetItemMetadata mengambil metadata item jika tersedia
// key: kunci unik yang mengidentifikasi item dalam metadata map
// Mengembalikan metadata item dan boolean yang menunjukkan apakah metadata ditemukan.
func (pm *PoolManager) GetItemMetadata(key string) (*PoolItemMetadata, bool) {
	metadata, ok := pm.itemMetadata.Load(key)
	if !ok {
		return nil, false
	}
	return metadata.(*PoolItemMetadata), true
}

func (pm *PoolManager) UpdateIdleDuration(key string) {
	pm.safelyUpdateMetadata(key, func(metadata *PoolItemMetadata) {
		if metadata.Status == "Idle" {
			metadata.IdleDuration = time.Since(metadata.LastUsed)
		}
	})
}

// safelyUpdateMetadata memperbarui metadata item secara aman menggunakan fungsi pembaruan yang diberikan
// key: kunci unik yang mengidentifikasi item dalam metadata map
// updateFunc: fungsi yang mendefinisikan bagaimana metadata harus diperbarui
// Fungsi ini memastikan bahwa metadata selalu diperbarui dengan cara yang aman
// menggunakan fungsi yang diberikan untuk memodifikasi metadata.
func (pm *PoolManager) safelyUpdateMetadata(key string, updateFunc func(*PoolItemMetadata)) {
	metadataVal, _ := pm.itemMetadata.LoadOrStore(key, &PoolItemMetadata{
		CreationTime: time.Now(),
		LastUsed:     time.Now(),
		Status:       "Active",
	})

	metadata := metadataVal.(*PoolItemMetadata)

	// Update metadata menggunakan fungsi yang diberikan
	updateFunc(metadata)

	// Simpan kembali hasil perubahan metadata ke dalam map
	pm.itemMetadata.Store(key, metadata)
}

func (pm *PoolManager) evictBatch(poolName string, batchSize int) {
	batch := make([]string, 0, batchSize)

	pm.itemMetadata.Range(func(key, value interface{}) bool {
		batch = append(batch, key.(string))

		// Jika batch sudah mencapai ukuran yang diinginkan, proses batch
		if len(batch) >= batchSize {
			pm.processEvictionBatch(poolName, batch)
			batch = batch[:0] // Reset batch
		}
		return true
	})

	// Proses sisa batch yang belum diproses
	if len(batch) > 0 {
		pm.processEvictionBatch(poolName, batch)
	}
}

func (pm *PoolManager) processEvictionBatch(poolName string, batch []string) {
	for _, key := range batch {
		pm.cache.Delete(key)
		pm.itemMetadata.Delete(key)
	}
	pm.logger.Printf("Evicted batch of items from pool: %s", poolName)
}

func (pm *PoolManager) removeItem(poolName, key string) {
	pm.cache.Delete(key)
	pm.itemMetadata.Delete(key)
	pm.logger.Printf("Removed item from pool: %s, Key: %s", poolName, key)
}

func (pm *PoolManager) safelyHandleInstance(poolName string, conf PoolConfiguration, instance PoolAble, action string) error {
	if action == "reset" {
		instance.Reset()
		pm.triggerCallbackWithInstance(conf.OnReset, poolName, instance)
	} else if action == "put" {
		pm.addToCache(poolName, instance)
		pm.triggerCallback(conf.OnPut, poolName)
	}
	return nil
}

func (pm *PoolManager) getPoolConfiguration(poolName string) (PoolConfiguration, error) {
	configVal, _ := pm.poolConfig.Load(poolName)
	conf, ok := configVal.(PoolConfiguration)
	if !ok {
		return PoolConfiguration{}, NewPoolError(poolName, "config", errors.New(ErrInvalidPoolConfigType))
	}
	return conf, nil
}

func (pm *PoolManager) updateMetadata(poolName, status string) {
	pm.safelyUpdateMetadata(poolName, func(metadata *PoolItemMetadata) {
		metadata.LastUsed = time.Now()
		metadata.Status = status
		metadata.Frequency++
	})
}

func (pm *PoolManager) triggerCallbackWithInstance(callback func(string, PoolAble), poolName string, instance PoolAble) {
	if callback != nil {
		callback(poolName, instance)
	}
}

func (pm *PoolManager) triggerCallback(callback func(string), poolName string) {
	if callback != nil {
		callback(poolName)
	}
}
