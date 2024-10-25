package poolmanager

import "sync"

type LocalCache struct {
	items map[string]interface{}
	mu    sync.RWMutex
}

func (lc *LocalCache) Get(key string) (interface{}, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	val, ok := lc.items[key]
	return val, ok
}

func (lc *LocalCache) Set(key string, value interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.items[key] = value
}
