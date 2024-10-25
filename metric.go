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
	LogLevel          LogLevel
	OnEvent           func(event PoolEvent)
}

type EventType int

const (
	EventAcquire EventType = iota
	EventRelease
	EventEvict
)

type PoolEvent struct {
	Type     EventType
	PoolName string
	Item     interface{}
}

func (pm *PoolManager) triggerEvent(event PoolEvent) {
	if pm.monitoringConfig.OnEvent != nil {
		pm.monitoringConfig.OnEvent(event)
	}
}

// GetPoolUsage mengakses metrik penggunaan pool secara langsung dari sync.Map.
func (pm *PoolManager) GetPoolUsage(poolType string) (int32, error) {
	if metrics, ok := pm.metrics.Load(poolType); ok {
		return metrics.(PoolMetrics).CurrentUsage, nil
	}
	return 0, errors.New("metrics not found for pool: " + poolType)
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

// getShardSize menghitung ukuran dari shard tertentu dalam sync.Pool
func (pm *PoolManager) getShardSize(poolType string, shardIndex int) int {
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

// getNonShardedPoolSize mengambil ukuran pool non-sharded di sync.Pool
func (pm *PoolManager) getNonShardedPoolSize(poolType string) int {
	size := 0
	pm.cache.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok && keyStr == poolType {
			size++
		}
		return true
	})
	return size
}
