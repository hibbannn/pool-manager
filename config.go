package poolmanager

import "time"

// PoolConfig digunakan untuk mengatur konfigurasi pool, seperti batas ukuran, auto-tuning, dan sharding
// Konfigurasi ini memungkinkan penyesuaian perilaku pool, termasuk pengaturan cache dan kebijakan eviksi.
type PoolConfig struct {
	SizeLimit        int                                // Batas maksimum jumlah objek dalam pool
	MinSize          int                                // Batas minimum jumlah objek dalam pool
	MaxSize          int                                // Batas maksimum ukuran pool saat auto-tuning
	InitialSize      int                                // Ukuran awal pool ketika diinisialisasi
	AutoTune         bool                               // Menentukan apakah auto-tuning diaktifkan atau tidak
	AutoTuneInterval time.Duration                      // Interval waktu untuk menjalankan auto-tuning
	AutoTuneFactor   float64                            // Faktor peningkatan ukuran saat auto-tuning diaktifkan
	EnableCaching    bool                               // Menentukan apakah caching diaktifkan
	CacheMaxSize     int                                // Batas maksimum jumlah objek dalam cache
	ShardingEnabled  bool                               // Menentukan apakah sharding diaktifkan
	ShardCount       int                                // Jumlah shard yang digunakan untuk sharding
	ShardStrategy    string                             // Strategi sharding yang digunakan (misalnya "hash", "round-robin", "random")
	TTL              time.Duration                      // Time-to-live untuk kebijakan eviksi pada objek yang tidak digunakan
	Eviction         EvictionPolicy                     // Kebijakan eviksi untuk menghapus objek dari pool
	EvictionInterval time.Duration                      // Interval waktu untuk menjalankan eviksi
	OnGet            func(poolType string)              // Callback yang dipanggil saat objek diambil dari pool
	OnPut            func(poolType string)              // Callback yang dipanggil saat objek dikembalikan ke pool
	OnEvict          func(poolType string)              // Callback yang dipanggil saat objek dihapus dari pool
	OnAutoTune       func(poolType string, newSize int) // Callback yang dipanggil saat auto-tuning terjadi
	OnError          func(poolType string, err error)   // Callback yang dipanggil saat terjadi error
}

// addToCache menambahkan instance ke dalam cache pool
// poolType: tipe pool yang digunakan untuk identifikasi cache
// instance: objek yang akan disimpan dalam cache
// Fungsi ini akan memeriksa konfigurasi pool untuk melihat apakah caching diaktifkan. Jika ukuran cache
// melebihi batas yang ditetapkan, fungsi ini akan menghapus item cache yang paling lama atau jarang digunakan.
func (pm *PoolManager) addToCache(poolType string, instance PoolAble) {
	// Load the pool configuration for the given pool type
	configVal, ok := pm.poolConfig.Load(poolType)
	if !ok {
		// Jika konfigurasi tidak ada, keluar dari fungsi
		return
	}

	// Melakukan type assertion untuk mendapatkan konfigurasi PoolConfig
	conf, ok := configVal.(PoolConfig)
	if !ok {
		// Jika type assertion gagal, keluar dari fungsi
		return
	}

	// Cek apakah caching diaktifkan
	if conf.EnableCaching {
		cacheSize := pm.getCacheSize(poolType)
		if cacheSize >= conf.CacheMaxSize {
			// Hapus item cache tertua atau LRU jika ukuran cache melebihi batas
			pm.evictOldestCacheItem(poolType)
		}
		// Simpan instance dalam cache
		pm.cache.Store(poolType, instance)
	}
}

// getCacheSize mendapatkan jumlah item dalam cache untuk tipe pool tertentu
// poolType: tipe pool yang digunakan untuk identifikasi cache
// Fungsi ini mengembalikan jumlah objek yang ada dalam cache untuk tipe pool yang diberikan.
func (pm *PoolManager) getCacheSize(poolType string) int {
	size := 0
	pm.cache.Range(func(key, value interface{}) bool {
		if key.(string) == poolType {
			size++
		}
		return true
	})
	return size
}
