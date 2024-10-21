package poolmanager

import (
	"math/rand"
	"sync/atomic"
)

type ShardingStrategy interface {
	GetShardIndex(poolType string, shardCount int, key string) int
}

// RoundRobinSharding implements round-robin strategy
type RoundRobinSharding struct {
	counter int64
}

func (rr *RoundRobinSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	return int(atomic.AddInt64(&rr.counter, 1) % int64(shardCount))
}

// RandomSharding implements random strategy
type RandomSharding struct{}

func (r *RandomSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	return rand.Intn(shardCount)
}

// HashSharding implements hash-based strategy
type HashSharding struct{}

func (h *HashSharding) GetShardIndex(poolType string, shardCount int, key string) int {
	hash := hashString(key)
	return int(hash % uint32(shardCount))
}
