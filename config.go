// Package poolmanager  adalah sebuah package di Go yang digunakan untuk mengelola pooling objek secara efisien. Package ini memungkinkan Anda untuk mengatur konfigurasi pooling, sharding, caching, auto-tuning, dan kebijakan eviksi untuk objek-objek yang sering digunakan dalam aplikasi Anda.
package poolmanager

import "time"

// PoolConfiguration digunakan untuk mengatur konfigurasi pool, seperti batas ukuran, auto-tuning, dan sharding
// Konfigurasi ini memungkinkan penyesuaian perilaku pool, termasuk pengaturan cache dan kebijakan eviksi.
// PoolConfiguration digunakan untuk mengatur konfigurasi pool, termasuk jenis key dan pemrosesannya
type PoolConfiguration struct {
	Name                  string                                   // Nama pool
	SizeLimit             int                                      // Batas maksimum jumlah objek dalam pool
	MinSize               int                                      // Batas minimum jumlah objek dalam pool
	MaxSize               int                                      // Batas maksimum ukuran pool saat auto-tuning
	InitialSize           int                                      // Ukuran awal pool ketika diinisialisasi
	AutoTune              bool                                     // Menentukan apakah auto-tuning diaktifkan atau tidak
	AutoTuneInterval      time.Duration                            // Interval waktu untuk menjalankan auto-tuning
	AutoTuneFactor        float64                                  // Faktor peningkatan ukuran saat auto-tuning diaktifkan
	AutoTuneDynamicFactor func(currentSize int) float64            // Fungsi dinamis untuk faktor auto-tuning
	EnableCaching         bool                                     // Menentukan apakah caching diaktifkan
	CacheMaxSize          int                                      // Batas maksimum jumlah objek dalam cache
	ShardingEnabled       bool                                     // Menentukan apakah sharding diaktifkan
	ShardCount            int                                      // Jumlah shard yang digunakan untuk sharding
	ShardStrategy         ShardingStrategy                         // Strategi sharding yang digunakan
	TTL                   time.Duration                            // Time-to-live untuk kebijakan eviksi pada objek yang tidak digunakan
	Eviction              EvictionPolicy                           // Kebijakan eviksi untuk menghapus objek dari pool
	EvictionInterval      time.Duration                            // Interval waktu untuk menjalankan eviksi
	KeyGenerator          func() string                            // Fungsi untuk menghasilkan kunci khusus
	OnGet                 func(poolType string)                    // Callback yang dipanggil saat objek diambil dari pool
	OnPut                 func(poolType string)                    // Callback yang dipanggil saat objek dikembalikan ke pool
	OnEvict               func(poolType string)                    // Callback yang dipanggil saat objek dihapus dari pool
	OnAutoTune            func(poolType string, newSize int)       // Callback yang dipanggil saat auto-tuning terjadi
	OnCreate              func(poolType string, instance PoolAble) // Callback yang dipanggil saat objek dibuat
	OnDestroy             func(poolType string, instance PoolAble) // Callback yang dipanggil saat objek dihancurkan
	OnReset               func(poolType string, instance PoolAble) // Callback yang dipanggil saat objek direset
	OnShard               func(poolType string, shardIndex int)    // Callback yang dipanggil saat sharding terjadi
	OnCacheHit            func(poolType string)                    // Callback yang dipanggil saat objek ditemukan
	OnError               func(poolType string, err error)         // Callback yang dipanggil saat terjadi error
}
