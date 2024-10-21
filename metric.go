package poolmanager

import (
	"errors"
	"sync/atomic"
)

// PoolMetrics untuk mencatat metrik penggunaan pool
// PoolMetrics menyimpan data mengenai jumlah operasi yang dilakukan pada pool,
// termasuk berapa kali objek diambil (TotalGets), dikembalikan (TotalPuts),
// dihapus (TotalEvicts), dan jumlah penggunaan pool saat ini (CurrentUsage).
type PoolMetrics struct {
	TotalGets    int64 // Total jumlah objek yang diambil dari pool
	TotalPuts    int64 // Total jumlah objek yang dikembalikan ke pool
	TotalEvicts  int64 // Total jumlah objek yang dihapus dari pool
	CurrentUsage int32 // Jumlah objek yang sedang digunakan
}

// MetricsCallback digunakan untuk mencatat metrik secara custom
// Callback ini memungkinkan pengguna untuk mencatat atau memonitor metrik
// penggunaan pool secara kustom berdasarkan tipe pool dan tindakan yang terjadi.
type MetricsCallback func(poolType, action string, metrics PoolMetrics)

// initMetrics menginisialisasi metrik untuk pool tertentu
// poolType: tipe pool untuk menginisialisasi metrik
// Fungsi ini digunakan untuk mempersiapkan penyimpanan metrik untuk sebuah pool,
// memastikan bahwa data metrik tersedia dan siap untuk dicatat.
func (pm *PoolManager) initMetrics(poolType string) {
	pm.metrics.Store(poolType, &PoolMetrics{})
}

// MonitoringConfig untuk mengatur konfigurasi monitoring
// MonitoringConfig menyediakan konfigurasi untuk logging dan pencatatan metrik
// kustom, termasuk apakah logging diaktifkan (EnableLogging), fungsi logging
// (LogFunc), dan fungsi pencatatan metrik kustom (CustomMetricsFunc).
type MonitoringConfig struct {
	EnableLogging     bool                 // Menentukan apakah logging diaktifkan
	LogFunc           func(message string) // Fungsi untuk mencatat log
	CustomMetricsFunc MetricsCallback      // Fungsi untuk mencatat metrik secara kustom
}

// GetMetrics memungkinkan pengguna untuk mendapatkan metrik pool saat ini
// poolType: tipe pool yang ingin diperiksa metriknya
// Mengembalikan objek PoolMetrics yang berisi informasi metrik pool, atau error
// jika metrik untuk tipe pool tersebut tidak ditemukan.
func (pm *PoolManager) GetMetrics(poolType string) (PoolMetrics, error) {
	metricsVal, ok := pm.metrics.Load(poolType)
	if !ok {
		return PoolMetrics{}, errors.New("metrics not found for pool: " + poolType)
	}
	return *(metricsVal.(*PoolMetrics)), nil
}

// recordMetric mencatat metrik penggunaan pool
// poolType: tipe pool yang metriknya akan dicatat
// action: tindakan yang dilakukan ("get", "put", atau "evict")
// Fungsi ini mencatat tindakan yang dilakukan pada pool dan memperbarui
// metrik secara atomik, untuk memastikan konsistensi data saat beberapa goroutine
// melakukan pencatatan secara bersamaan.
func (pm *PoolManager) recordMetric(poolType, action string) {
	// Memastikan metrik sudah ada, jika tidak, buat baru
	metricsVal, _ := pm.metrics.LoadOrStore(poolType, &PoolMetrics{})
	metrics, ok := metricsVal.(*PoolMetrics)
	if !ok {
		return
	}

	// Memperbarui metrik secara atomik
	switch action {
	case "get":
		atomic.AddInt64(&metrics.TotalGets, 1)
		atomic.AddInt32(&metrics.CurrentUsage, 1)
	case "put":
		atomic.AddInt64(&metrics.TotalPuts, 1)
		atomic.AddInt32(&metrics.CurrentUsage, -1)
	case "evict":
		atomic.AddInt64(&metrics.TotalEvicts, 1)
	}
}

// getCurrentUsage mendapatkan jumlah penggunaan pool saat ini
// poolType: tipe pool yang ingin diperiksa jumlah penggunaannya
// Mengembalikan jumlah objek yang sedang digunakan dalam pool saat ini.
func (pm *PoolManager) getCurrentUsage(poolType string) int32 {
	metricsVal, ok := pm.metrics.Load(poolType)
	if !ok {
		return 0
	}
	metrics, ok := metricsVal.(*PoolMetrics)
	if !ok {
		return 0
	}
	return metrics.CurrentUsage
}
