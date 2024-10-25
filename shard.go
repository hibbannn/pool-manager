package poolmanager

import (
	"math/rand"
	"sync/atomic"
	"time"
)

type ShardingStrategy interface {
	GetShardIndex(poolType string, shardCount int, key string) int
}

// RoundRobinSharding implements round-robin strategy
type RoundRobinSharding struct {
	counter int64
}

func (rr *RoundRobinSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	// Contoh: Menerapkan logika berbeda berdasarkan poolType
	if poolType == "specialPool" {
		// Jika poolType tertentu, gunakan beberapa langkah tambahan
		return int(atomic.AddInt64(&rr.counter, 3) % int64(shardCount))
	}
	// Default ke round-robin biasa
	return int(atomic.AddInt64(&rr.counter, 1) % int64(shardCount))
}

// RandomSharding implements random strategy
type RandomSharding struct {
	rng *rand.Rand // Generator acak lokal
}

// NewRandomSharding membuat instance baru dari RandomSharding
func NewRandomSharding() *RandomSharding {
	return &RandomSharding{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())), // Inisialisasi dengan seed berdasarkan waktu saat ini
	}
}

func (r *RandomSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	// Contoh: Jika poolType adalah "specialPool", maka modifikasi generator acak lokal
	if poolType == "specialPool" {
		r.rng = rand.New(rand.NewSource(int64(len(poolType))))
	}
	return r.rng.Intn(shardCount)
}

// HashSharding implements hash-based strategy
type HashSharding struct{}

func (h *HashSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	// Contoh: Menggunakan kombinasi poolType dan key untuk hashing
	hash := hashString(poolType + key)
	return int(hash % uint32(shardCount))
}
