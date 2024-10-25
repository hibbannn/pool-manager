package poolmanager

import (
	"errors"
	"time"
)

// PoolConfigBuilder digunakan untuk membangun konfigurasi pool menggunakan pola builder dengan chaining.
type PoolConfigBuilder struct {
	config PoolConfiguration
}

// NewPoolConfiguration membuat instance PoolConfigBuilder baru dengan konfigurasi default minimal.
// Menetapkan beberapa nilai default, seperti ukuran pool dan pengaturan lainnya.
func NewPoolConfiguration(poolName string) *PoolConfigBuilder {
	return &PoolConfigBuilder{config: PoolConfiguration{
		Name:             poolName,
		SizeLimit:        10,               // Default minimal size limit
		MinSize:          1,                // Ukuran minimal pool
		MaxSize:          10,               // Ukuran maksimal pool
		InitialSize:      1,                // Ukuran awal yang sangat kecil
		AutoTune:         false,            // Auto-tuning tidak diaktifkan secara default
		AutoTuneFactor:   1.0,              // Faktor auto-tuning default
		EnableCaching:    false,            // Caching tidak diaktifkan secara default
		CacheMaxSize:     5,                // Ukuran cache minimal
		ShardingEnabled:  false,            // Sharding tidak diaktifkan secara default
		ShardCount:       1,                // Jumlah shard default minimal
		TTL:              time.Minute * 5,  // Time-to-live default minimal
		EvictionInterval: time.Minute * 10, // Interval eviksi default
	}}
}

// WithSizeLimit menetapkan batas maksimum jumlah objek yang dapat disimpan dalam pool.
func (b *PoolConfigBuilder) WithSizeLimit(sizeLimit int) *PoolConfigBuilder {
	b.config.SizeLimit = sizeLimit
	return b
}

// WithInitialSize menetapkan ukuran awal pool saat diinisialisasi.
func (b *PoolConfigBuilder) WithInitialSize(initialSize int) *PoolConfigBuilder {
	b.config.InitialSize = initialSize
	return b
}

func (b *PoolConfigBuilder) WithMinSize(minSize int) *PoolConfigBuilder {
	b.config.MinSize = minSize
	return b
}

func (b *PoolConfigBuilder) WithMaxSize(maxSize int) *PoolConfigBuilder {
	b.config.MaxSize = maxSize
	return b
}

func (b *PoolConfigBuilder) WithOnReset(onReset func(poolType string, instance PoolAble)) *PoolConfigBuilder {
	b.config.OnReset = onReset
	return b
}
func (b *PoolConfigBuilder) WithOnCreate(onCreate func(poolType string, instance PoolAble)) *PoolConfigBuilder {
	b.config.OnCreate = onCreate
	return b
}

func (b *PoolConfigBuilder) WithOnGet(onGet func(poolType string)) *PoolConfigBuilder {
	b.config.OnGet = onGet
	return b
}

func (b *PoolConfigBuilder) WithOnPut(onPut func(poolType string)) *PoolConfigBuilder {
	b.config.OnPut = onPut
	return b
}

func (b *PoolConfigBuilder) WithOnAutoTune(onAutoTune func(poolType string, newSize int)) *PoolConfigBuilder {
	b.config.OnAutoTune = onAutoTune
	return b
}

// WithAutoTune mengaktifkan atau menonaktifkan auto-tuning pada pool.
func (b *PoolConfigBuilder) WithAutoTune(autoTune bool) *PoolConfigBuilder {
	b.config.AutoTune = autoTune
	return b
}

// WithAutoTuneFactor menetapkan faktor peningkatan ukuran saat auto-tuning.
func (b *PoolConfigBuilder) WithAutoTuneFactor(factor float64) *PoolConfigBuilder {
	b.config.AutoTuneFactor = factor
	return b
}

// WithSharding mengaktifkan atau menonaktifkan sharding.
func (b *PoolConfigBuilder) WithSharding(enabled bool, shardCount int) *PoolConfigBuilder {
	b.config.ShardingEnabled = enabled
	b.config.ShardCount = shardCount
	return b
}

// WithTTL menetapkan Time-to-Live (TTL) untuk kebijakan eviksi pada pool.
func (b *PoolConfigBuilder) WithTTL(ttl time.Duration) *PoolConfigBuilder {
	b.config.TTL = ttl
	return b
}

// WithEnableCaching mengaktifkan atau menonaktifkan caching pada pool.
func (b *PoolConfigBuilder) WithEnableCaching(enableCaching bool) *PoolConfigBuilder {
	b.config.EnableCaching = enableCaching
	return b
}

// WithCacheMaxSize menetapkan ukuran maksimum cache yang dapat digunakan.
func (b *PoolConfigBuilder) WithCacheMaxSize(cacheMaxSize int) *PoolConfigBuilder {
	b.config.CacheMaxSize = cacheMaxSize
	return b
}

// WithEvictionInterval menetapkan interval waktu untuk menjalankan eviksi pada pool.
func (b *PoolConfigBuilder) WithEvictionInterval(evictionInterval time.Duration) *PoolConfigBuilder {
	b.config.EvictionInterval = evictionInterval
	return b
}

// WithEvictionPolicy menetapkan kebijakan eviksi yang digunakan.
func (b *PoolConfigBuilder) WithEvictionPolicy(evictionPolicy EvictionPolicy) *PoolConfigBuilder {
	b.config.Eviction = evictionPolicy
	return b
}

// Build menghasilkan objek PoolConfiguration berdasarkan konfigurasi yang telah diatur pada builder.
func (b *PoolConfigBuilder) Build() (PoolConfiguration, error) {
	if err := b.config.Validate(); err != nil {
		return PoolConfiguration{}, err
	}
	return b.config, nil
}

// Validate memeriksa apakah konfigurasi pool valid.
func (config *PoolConfiguration) Validate() error {
	if config.SizeLimit < 0 {
		return errors.New("SizeLimit must be non-negative")
	}
	if config.MinSize < 0 || config.MaxSize < 0 {
		return errors.New("MinSize and MaxSize must be non-negative")
	}
	if config.MaxSize < config.MinSize {
		return errors.New("MaxSize cannot be less than MinSize")
	}
	if config.InitialSize < config.MinSize || config.InitialSize > config.MaxSize {
		return errors.New("InitialSize must be between MinSize and MaxSize")
	}
	if config.ShardingEnabled && config.ShardCount <= 1 {
		return errors.New("ShardCount must be greater than 1 if ShardingEnabled is true")
	}
	if config.AutoTune && config.AutoTuneFactor <= 0 {
		return errors.New("AutoTuneFactor must be greater than 0")
	}
	return nil
}
