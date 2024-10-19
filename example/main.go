package main

import (
	"fmt"
	"github.com/hibbannn/pool-manager"
	"sync"
	"time"
)

// LargeObject adalah Contoh struct yang mengimplementasikan PoolAble, dengan objek yang lebih besar
type LargeObject struct {
	Data   [102400]byte // Simulasi objek yang lebih besar (100 KB)
	Values [500]int
	ID     int
	Name   string
}

// Reset mengimplementasikan interface PoolAble (Ini Wajib dibuat jika ingin menggunakan Package PoolManager)
func (l *LargeObject) Reset() {
	l.ID = 0
	l.Name = ""
}

// Function untuk mengukur waktu yang dibutuhkan oleh suatu operasi
func measureTime(name string, operation func()) {
	start := time.Now()
	operation()
	duration := time.Since(start)
	fmt.Printf("%s selesai dalam %v\n", name, duration)
}

func main() {
	// Buat instance PoolManager
	poolManager := poolmanager.NewPoolManager()

	// Konfigurasi pool dengan berbagai fitur
	poolConfig := poolmanager.NewPoolConfigBuilder().
		WithSizeLimit(200000).
		WithInitialSize(10000).
		WithAutoTune(true).
		WithAutoTuneFactor(1.5).
		WithSharding(true, 10).
		WithEnableCaching(true).  // Aktifkan caching
		WithCacheMaxSize(5000).   // Batas maksimum cache
		WithTTL(2 * time.Minute). // Time-to-live untuk eviksi otomatis
		Build()

	// Tambahkan pool dengan tipe "largeObject" dan factory function
	err := poolManager.AddPool("largeObject", func() poolmanager.PoolAble {
		return &LargeObject{}
	}, poolConfig)

	if err != nil {
		fmt.Println("Gagal menambahkan pool:", err)
		return
	}

	// Tambahkan konfigurasi monitoring dan callback untuk mencatat aktivitas
	poolManager.SetMonitoringConfig(poolmanager.MonitoringConfig{
		EnableLogging: true,
		LogFunc: func(message string) {
			fmt.Println("Log:", message)
		},
	})

	// Tentukan jumlah worker dan jumlah iterasi per worker
	const numWorkers = 10
	const iterations = 100000

	// Uji dengan pooling
	measureTime("Test dengan Pooling", func() {
		var wg sync.WaitGroup
		wg.Add(numWorkers)

		for w := 0; w < numWorkers; w++ {
			go func(workerID int) {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					instance, err := poolManager.AcquireInstance("largeObject")
					if err != nil {
						fmt.Println("Gagal mengambil instance:", err)
						continue
					}

					// Casting ke LargeObject dan lakukan operasi
					largeObj, ok := instance.(*LargeObject)
					if ok {
						largeObj.ID = workerID*iterations + i
						largeObj.Name = fmt.Sprintf("Worker-%d-Item-%d", workerID, i)
						// Simulasi penulisan data
						largeObj.Data[0] = byte(workerID)
						largeObj.Values[0] = i

						// Menampilkan beberapa data untuk verifikasi
						if i%10000 == 0 { // Tampilkan setiap 10.000 iterasi
							fmt.Printf("Use Pool Manager - Worker %d: ID=%d, Name=%s, Data[0]=%d, Values[0]=%d\n",
								workerID, largeObj.ID, largeObj.Name, largeObj.Data[0], largeObj.Values[0])
						}
					}

					// Kembalikan instance ke pool
					err = poolManager.ReleaseInstance("largeObject", largeObj)
					if err != nil {
						fmt.Println("Gagal mengembalikan instance:", err)
					}
				}
			}(w)
		}

		wg.Wait()
	})

	// Uji tanpa pooling
	measureTime("Test tanpa Pooling", func() {
		var wg sync.WaitGroup
		wg.Add(numWorkers)

		for w := 0; w < numWorkers; w++ {
			go func(workerID int) {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					// Buat objek baru tanpa menggunakan pool
					largeObj := &LargeObject{
						ID:   workerID*iterations + i,
						Name: fmt.Sprintf("Worker-%d-Item-%d", workerID, i),
					}
					// Simulasi penulisan data
					largeObj.Data[0] = byte(workerID)
					largeObj.Values[0] = i

					// Menampilkan beberapa data untuk verifikasi
					if i%10000 == 0 { // Tampilkan setiap 10.000 iterasi
						fmt.Printf("No Pooling - Worker %d: ID=%d, Name=%s, Data[0]=%d, Values[0]=%d\n",
							workerID, largeObj.ID, largeObj.Name, largeObj.Data[0], largeObj.Values[0])
					}
				}
			}(w)
		}

		wg.Wait()
	})

	// Hapus pool saat tidak digunakan lagi
	err = poolManager.RemovePool("largeObject")
	if err != nil {
		fmt.Println("Gagal menghapus pool:", err)
	}

	fmt.Println("Selesai menjalankan tes kecepatan.")
}
