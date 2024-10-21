package main

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/hibbannn/pool-manager" // Ganti dengan path package yang sesuai
)

// MyObject adalah contoh implementasi PoolAble dengan ukuran 100 KB
type MyObject struct {
	Data []byte // Data dengan ukuran sekitar 100 KB
}

// Reset mengimplementasikan metode Reset untuk MyObject
func (obj *MyObject) Reset() {
	for i := range obj.Data {
		obj.Data[i] = 0
	}
}

// CustomSharding adalah strategi sharding kustom
type CustomSharding struct{}

// GetShardIndex mengimplementasikan sharding kustom berdasarkan panjang kunci
func (cs *CustomSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	return len(key) % shardCount // Strategi sederhana berdasarkan panjang kunci
}

func main() {
	fmt.Println("=== Perbandingan Pooling vs Tanpa Pooling ===")

	// Pengujian dengan PoolManager menggunakan berbagai strategi sharding
	fmt.Println("\n--- Dengan PoolManager (Round-Robin Sharding) ---")
	testWithPoolManager("round-robin")

	fmt.Println("\n--- Dengan PoolManager (Random Sharding) ---")
	testWithPoolManager("random")

	fmt.Println("\n--- Dengan PoolManager (Hash Sharding) ---")
	testWithPoolManager("hash")

	fmt.Println("\n--- Dengan PoolManager (Custom Sharding) ---")
	testWithPoolManager("custom")

	// Pengujian tanpa PoolManager
	fmt.Println("\n--- Tanpa PoolManager ---")
	testWithoutPoolManager()
}

// testWithPoolManager melakukan pengujian dengan menggunakan PoolManager
func testWithPoolManager(shardingType string) {
	pm := poolmanager.NewPoolManager()

	// Pilih strategi sharding yang digunakan
	var shardingStrategy poolmanager.ShardingStrategy
	switch shardingType {
	case "round-robin":
		shardingStrategy = &poolmanager.RoundRobinSharding{}
	case "random":
		shardingStrategy = &poolmanager.RandomSharding{}
	case "hash":
		shardingStrategy = &poolmanager.HashSharding{}
	case "custom":
		shardingStrategy = &CustomSharding{}
	default:
		shardingStrategy = &poolmanager.RoundRobinSharding{} // Default ke round-robin
	}

	// Konfigurasi pool
	poolConfig := poolmanager.PoolConfig{
		SizeLimit:       1000,
		InitialSize:     50,
		AutoTune:        false,
		ShardingEnabled: false,
		ShardCount:      12,
		ShardStrategy:   shardingStrategy,
		EnableCaching:   false,
		CacheMaxSize:    50,
		TTL:             10 * time.Second,
	}

	// Tambahkan pool dengan konfigurasi
	factoryFunc := func() poolmanager.PoolAble {
		return &MyObject{Data: make([]byte, 200*1024)} // Membuat objek dengan ukuran sekitar 200 KB
	}
	err := pm.AddPool("my_pool", factoryFunc, poolConfig)
	if err != nil {
		log.Fatalf("Gagal menambahkan pool: %v", err)
	}

	// Mengukur waktu dan penggunaan memori
	start := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Ambil dan kembalikan objek 1000 kali
	for i := 0; i < 1000; i++ {
		obj, err := pm.AcquireInstance("my_pool")
		if err != nil {
			log.Printf("Gagal mendapatkan instance: %v", err)
			continue
		}

		// Simulasi penggunaan objek
		myObj := obj.(*MyObject)
		myObj.Data[rand.Intn(100*1024)] = byte(rand.Intn(256))

		// Kembalikan objek ke pool
		err = pm.ReleaseInstance("my_pool", myObj)
		if err != nil {
			log.Printf("Gagal mengembalikan instance: %v", err)
		}
	}

	runtime.ReadMemStats(&memAfter)
	elapsed := time.Since(start)

	// Cetak hasil
	fmt.Printf("Waktu yang diperlukan: %v\n", elapsed)
	fmt.Printf("Memori sebelum: %v KB\n", memBefore.Alloc/1024)
	fmt.Printf("Memori setelah: %v KB\n", memAfter.Alloc/1024)

	// Tampilkan konfigurasi pool yang digunakan
	fmt.Println("\nKonfigurasi Pool yang Digunakan:")
	fmt.Printf("Sharding Type: %s\nSize Limit: %d\nInitial Size: %d\nAuto Tune: %t\nSharding Enabled: %t\nShard Count: %d\n",
		shardingType, poolConfig.SizeLimit, poolConfig.InitialSize, poolConfig.AutoTune, poolConfig.ShardingEnabled, poolConfig.ShardCount)
	fmt.Printf("Caching Enabled: %t\nCache Max Size: %d\nTTL: %s\n",
		poolConfig.EnableCaching, poolConfig.CacheMaxSize, poolConfig.TTL)
}

// testWithoutPoolManager melakukan pengujian tanpa menggunakan PoolManager
func testWithoutPoolManager() {
	// Mengukur waktu dan penggunaan memori
	start := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Alokasi dan de-alokasi objek 1000 kali
	for i := 0; i < 1000; i++ {
		// Membuat objek baru dengan ukuran sekitar 100 KB
		obj := &MyObject{Data: make([]byte, 200*1024)}

		// Simulasi penggunaan objek
		obj.Data[rand.Intn(200*1024)] = byte(rand.Intn(256))

		// Pengembalian objek dilakukan secara manual dengan membiarkan objek tersebut
		// didaur ulang oleh garbage collector
	}

	runtime.ReadMemStats(&memAfter)
	elapsed := time.Since(start)

	// Cetak hasil
	fmt.Printf("Waktu yang diperlukan: %v\n", elapsed)
	fmt.Printf("Memori sebelum: %v KB\n", memBefore.Alloc/1024)
	fmt.Printf("Memori setelah: %v KB\n", memAfter.Alloc/1024)
}
