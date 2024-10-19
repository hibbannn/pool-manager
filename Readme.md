# PoolManager

`poolmanager` adalah sebuah package di Go yang digunakan untuk mengelola pooling objek secara efisien. Package ini memungkinkan Anda untuk mengatur konfigurasi pooling, sharding, caching, auto-tuning, dan kebijakan eviksi untuk objek-objek yang sering digunakan dalam aplikasi Anda.

## Fitur

- **Pooling**: Mengelola objek secara efisien untuk menghindari overhead pembuatan objek berulang.
- **Sharding**: Mendukung pembagian pool menjadi beberapa shard untuk meningkatkan performa pada lingkungan bersamaan.
- **Caching**: Menyediakan caching untuk mengurangi akses berulang ke objek yang sering digunakan.
- **Auto-Tuning**: Menyesuaikan ukuran pool secara otomatis berdasarkan kebutuhan aplikasi.
- **Eviction Policies**: Mendukung berbagai kebijakan eviksi seperti TTL, LRU, dan LFU untuk mengelola objek yang tidak terpakai.
- **Monitoring**: Mencatat metrik penggunaan objek dalam pool dan menyediakan callback untuk berbagai kejadian.

## Instalasi

Untuk menggunakan package ini, silakan unduh melalui Go modules:

```bash
go get github.com/hibbannn/poolmanager
```

## Pool Configuration Guide

Anda dapat mengatur konfigurasi pool menggunakan `PoolConfigBuilder`. Berikut adalah opsi konfigurasi yang tersedia:

### Opsi Konfigurasi

#### `WithSizeLimit(sizeLimit int)`
- Menetapkan batas maksimum ukuran pool.
- **Parameter:**
    - `sizeLimit`: Batas maksimum jumlah objek yang dapat disimpan di dalam pool.

#### `WithInitialSize(initialSize int)`
- Menetapkan ukuran awal pool saat diinisialisasi.
- **Parameter:**
    - `initialSize`: Ukuran awal objek di dalam pool.

#### `WithAutoTune(autoTune bool)`
- Mengaktifkan atau menonaktifkan fitur auto-tuning.
- **Parameter:**
    - `autoTune`: `true` untuk mengaktifkan auto-tuning, `false` untuk menonaktifkan.

#### `WithAutoTuneFactor(factor float64)`
- Menetapkan faktor peningkatan ukuran pool saat auto-tuning diaktifkan.
- **Parameter:**
    - `factor`: Faktor peningkatan ukuran.

#### `WithEnableCaching(enable bool)`
- Mengaktifkan atau menonaktifkan caching.
- **Parameter:**
    - `enable`: `true` untuk mengaktifkan caching, `false` untuk menonaktifkan.

#### `WithCacheMaxSize(maxSize int)`
- Menetapkan batas maksimum jumlah objek yang dapat disimpan dalam cache.
- **Parameter:**
    - `maxSize`: Batas maksimum ukuran cache.

#### `WithSharding(enabled bool, shardCount int)`
- Mengaktifkan fitur sharding dan menetapkan jumlah shard.
- **Parameter:**
    - `enabled`: `true` untuk mengaktifkan sharding, `false` untuk menonaktifkan.
    - `shardCount`: Jumlah shard yang digunakan.

#### `WithTTL(ttl time.Duration)`
- Menetapkan Time-to-Live (TTL) untuk kebijakan eviksi objek dalam pool.
- **Parameter:**
    - `ttl`: Durasi TTL.

#### `WithOnGet(callback func(poolType string))`
- Menetapkan callback yang dipanggil saat objek diambil dari pool.
- **Parameter:**
    - `callback`: Fungsi yang dipanggil, dengan parameter `poolType` yang menunjukkan tipe pool.

#### `WithOnPut(callback func(poolType string))`
- Menetapkan callback yang dipanggil saat objek dikembalikan ke pool.
- **Parameter:**
    - `callback`: Fungsi yang dipanggil, dengan parameter `poolType` yang menunjukkan tipe pool.

#### `WithOnEvict(callback func(poolType string))`
- Menetapkan callback yang dipanggil saat objek dihapus dari pool.
- **Parameter:**
    - `callback`: Fungsi yang dipanggil, dengan parameter `poolType` yang menunjukkan tipe pool.

#### `WithOnAutoTune(callback func(poolType string, newSize int))`
- Menetapkan callback yang dipanggil saat auto-tuning terjadi.
- **Parameter:**
    - `callback`: Fungsi yang dipanggil, dengan parameter `poolType` dan `newSize` yang menunjukkan ukuran baru setelah auto-tuning.

#### `WithOnError(callback func(poolType string, err error))`
- Menetapkan callback yang dipanggil saat terjadi error.
- **Parameter:**
    - `callback`: Fungsi yang dipanggil, dengan parameter `poolType` dan `err` yang menunjukkan jenis kesalahan.

## Contoh Builder

Berikut adalah contoh penggunaan konfigurasi pool:

```go
poolConfig := NewPoolConfigBuilder().
    WithSizeLimit(100).
    WithInitialSize(10).
    WithAutoTune(true).
    WithAutoTuneFactor(1.5).
    WithEnableCaching(true).
    WithCacheMaxSize(50).
    WithSharding(true, 4).
    WithTTL(5 * time.Minute).
    WithOnGet(func(poolType string) {
        fmt.Println("Object taken from pool:", poolType)
    }).
    WithOnPut(func(poolType string) {
        fmt.Println("Object returned to pool:", poolType)
    }).
    WithOnEvict(func(poolType string) {
        fmt.Println("Object evicted from pool:", poolType)
    }).
    WithOnAutoTune(func(poolType string, newSize int) {
        fmt.Printf("Auto-tuning for %s, new size: %d\n", poolType, newSize)
    }).
    WithOnError(func(poolType string, err error) {
        fmt.Printf("Error in %s: %v\n", poolType, err)
    }).
    Build()
```

## Mengimplementasikan `PoolAble`

Untuk menggunakan objek dalam pool, struct harus mengimplementasikan interface `PoolAble` dengan mendefinisikan metode `Reset`. Metode ini digunakan untuk mereset status objek sebelum dikembalikan ke pool.
Contoh:
```go
type LargeObject struct {
    Data   [102400]byte // Simulasi objek besar (100 KB)
    ID     int
    Name   string
}

// Reset mengimplementasikan metode Reset pada interface PoolAble
func (l *LargeObject) Reset() {
    l.ID = 0
    l.Name = ""
}
```

### Sharding

Sharding dapat diaktifkan untuk membagi pool menjadi beberapa bagian (shard) yang berbeda. Ini berguna dalam aplikasi bersamaan dengan tingkat konkurensi tinggi, di mana akses ke objek dari pool sering terjadi. Sharding membantu mengurangi kontensi dengan mendistribusikan permintaan ke beberapa shard.

- **WithSharding**: Mengaktifkan fitur sharding dan menentukan jumlah shard.

Contoh:
```go
WithSharding(true, 4) // Mengaktifkan sharding dengan 4 shard
```

### Kebijakan Eviksi

`poolmanager` mendukung beberapa kebijakan eviksi untuk mengelola objek dalam pool, termasuk:

- **TTL (Time-To-Live)**: Menghapus objek yang sudah tidak digunakan selama periode waktu tertentu.
- **LRU (Least Recently Used)**: Menghapus objek yang paling jarang digunakan baru-baru ini.
- **LFU (Least Frequently Used)**: Menghapus objek yang paling jarang digunakan secara keseluruhan.

Anda dapat menetapkan kebijakan eviksi melalui konfigurasi pool.

Contoh:
```go
poolConfig := NewPoolConfigBuilder().
    WithTTL(5 * time.Minute).
    Build()
```

## Contoh
```go
package main

import (
    "fmt"
    "sync"
    "time"
    "github.com/hibbannn/poolmanager/pkg"
)

// LargeObject adalah contoh struct yang mengimplementasikan interface PoolAble
type LargeObject struct {
    Data   [102400]byte // Simulasi objek besar (100 KB)
    ID     int
    Name   string
}

// Reset mengimplementasikan metode Reset pada interface PoolAble
func (l *LargeObject) Reset() {
    l.ID = 0
    l.Name = ""
}

func main() {
    // Buat instance PoolManager
    poolManager := poolmanager.NewPoolManager()

    // Konfigurasi Pool dengan berbagai opsi
    poolConfig := poolmanager.NewPoolConfigBuilder().
        WithSizeLimit(2000).
        WithInitialSize(100).
        WithAutoTune(true).
        WithAutoTuneFactor(1.5).
        WithEnableCaching(true).
        WithCacheMaxSize(500).
        WithSharding(true, 10).
        WithTTL(10 * time.Minute).
        Build()

    // Tambahkan pool dengan tipe "largeObject"
    err := poolManager.AddPool("largeObject", func() poolmanager.PoolAble {
        return &LargeObject{}
    }, poolConfig)

    if err != nil {
        fmt.Println("Gagal menambahkan pool:", err)
        return
    }

    // Contoh penggunaan pool dengan beberapa goroutine
    const numWorkers = 10
    const iterations = 1000
    var wg sync.WaitGroup
    wg.Add(numWorkers)

    for i := 0; i < numWorkers; i++ {
        go func(workerID int) {
            defer wg.Done()
            for j := 0; j < iterations; j++ {
                instance, err := poolManager.AcquireInstance("largeObject")
                if err != nil {
                    fmt.Println("Gagal mengambil instance:", err)
                    continue
                }

                largeObj, ok := instance.(*LargeObject)
                if ok {
                    largeObj.ID = workerID*iterations + j
                    largeObj.Name = fmt.Sprintf("Worker-%d-Item-%d", workerID, j)
                }

                // Kembalikan objek ke pool setelah selesai digunakan
                err = poolManager.ReleaseInstance("largeObject", largeObj)
                if err != nil {
                    fmt.Println("Gagal mengembalikan instance:", err)
                }
            }
        }(i)
    }

    wg.Wait()
    fmt.Println("Selesai menjalankan contoh.")
}
```

### FAQ / Troubleshooting

#### Q: Mengapa saya mendapatkan error "pool does not exist" saat memanggil `AcquireInstance`?
A: Pastikan Anda sudah menambahkan pool dengan `AddPool` sebelum mencoba mengambil instance dari pool.

#### Q: Apa yang terjadi jika `ReleaseInstance` gagal?
A: Jika terjadi error saat mengembalikan objek ke pool, periksa callback `OnError` untuk menangani error ini dengan lebih baik.
