package poolmanager

import "time"

// PoolConfigBuilder digunakan untuk membangun konfigurasi pool menggunakan pola builder dengan chaining.
type PoolConfigBuilder struct {
	config PoolConfig
}

// NewPoolConfigBuilder membuat instance PoolConfigBuilder baru untuk memulai proses konfigurasi pool.
func NewPoolConfigBuilder() *PoolConfigBuilder {
	return &PoolConfigBuilder{config: PoolConfig{}}
}

// WithSizeLimit menetapkan batas maksimum ukuran pool,
// sizeLimit: jumlah maksimum objek yang dapat disimpan dalam pool.
func (b *PoolConfigBuilder) WithSizeLimit(sizeLimit int) *PoolConfigBuilder {
	b.config.SizeLimit = sizeLimit
	return b
}

// WithInitialSize menetapkan ukuran awal pool saat diinisialisasi,
// initialSize: jumlah objek yang dimasukkan ke dalam pool saat diinisialisasi.
func (b *PoolConfigBuilder) WithInitialSize(initialSize int) *PoolConfigBuilder {
	b.config.InitialSize = initialSize
	return b
}

// WithAutoTune mengaktifkan atau menonaktifkan auto-tuning pada pool,
// autoTune: jika true, maka auto-tuning akan diaktifkan.
func (b *PoolConfigBuilder) WithAutoTune(autoTune bool) *PoolConfigBuilder {
	b.config.AutoTune = autoTune
	return b
}

// WithAutoTuneFactor menetapkan faktor peningkatan ukuran saat auto-tuning,
// factor: faktor yang digunakan untuk menyesuaikan ukuran pool ketika auto-tuning diaktifkan.
func (b *PoolConfigBuilder) WithAutoTuneFactor(factor float64) *PoolConfigBuilder {
	b.config.AutoTuneFactor = factor
	return b
}

// WithSharding mengaktifkan atau menonaktifkan sharding,
// enabled: jika true, maka sharding akan diaktifkan,
// shardCount: jumlah shard yang digunakan jika sharding diaktifkan.
func (b *PoolConfigBuilder) WithSharding(enabled bool, shardCount int) *PoolConfigBuilder {
	b.config.ShardingEnabled = enabled
	b.config.ShardCount = shardCount
	return b
}

// WithTTL menetapkan Time-to-Live (TTL) untuk kebijakan eviksi pada pool,
// ttl: durasi sebelum objek dihapus dari pool.
func (b *PoolConfigBuilder) WithTTL(ttl time.Duration) *PoolConfigBuilder {
	b.config.TTL = ttl
	return b
}

// WithEnableCaching mengaktifkan atau menonaktifkan caching pada pool,
// enableCaching: jika true, maka caching akan diaktifkan.
func (b *PoolConfigBuilder) WithEnableCaching(enableCaching bool) *PoolConfigBuilder {
	b.config.EnableCaching = enableCaching
	return b
}

// WithCacheMaxSize menetapkan ukuran maksimum cache yang dapat digunakan,
// cacheMaxSize: jumlah maksimum objek yang dapat disimpan dalam cache.
func (b *PoolConfigBuilder) WithCacheMaxSize(cacheMaxSize int) *PoolConfigBuilder {
	b.config.CacheMaxSize = cacheMaxSize
	return b
}

// WithEvictionInterval menetapkan interval waktu untuk menjalankan eviksi pada pool,
// evictionInterval: durasi interval untuk eviksi.
func (b *PoolConfigBuilder) WithEvictionInterval(evictionInterval time.Duration) *PoolConfigBuilder {
	b.config.EvictionInterval = evictionInterval
	return b
}

// WithEvictionPolicy menetapkan kebijakan eviksi yang digunakan
// evictionPolicy: kebijakan eviksi yang digunakan untuk pool
func (b *PoolConfigBuilder) WithEvictionPolicy(evictionPolicy EvictionPolicy) *PoolConfigBuilder {
	b.config.Eviction = evictionPolicy
	return b
}

// Build menghasilkan objek PoolConfig berdasarkan konfigurasi yang telah diatur pada builder
func (b *PoolConfigBuilder) Build() PoolConfig {
	return b.config
}
