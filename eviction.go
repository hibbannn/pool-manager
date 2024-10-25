package poolmanager

import (
	"time"
)

// EvictionPolicy interface untuk kebijakan eviksi
// EvictionPolicy mendefinisikan metode ShouldEvict, yang digunakan untuk menentukan
// apakah suatu objek dalam pool harus dihapus berdasarkan kebijakan eviksi tertentu.
type EvictionPolicy interface {
	// ShouldEvict mengevaluasi apakah objek harus dieviksikan
	// key: kunci unik dari objek yang dievaluasi
	// metadata: metadata dari objek yang digunakan untuk mengevaluasi kebijakan eviksi
	// Mengembalikan nilai true jika objek harus dieviksikan, false jika tidak.
	ShouldEvict(key string, metadata *PoolItemMetadata) bool

	// Evict mengevaluasi apakah objek harus dieviksikan
	// poolType: tipe pool dari mana item akan dihapus
	// Fungsi ini mencari item dengan waktu terakhir digunakan paling lama dan menghapusnya dari cache dan metadata.
	Evict(poolType string, pm *PoolManager)
}

// Implementasi Evict untuk SmartEvictionPolicy
func (p *SmartEvictionPolicy) Evict(poolType string, pm *PoolManager) {
	pm.itemMetadata.Range(func(key, value interface{}) bool {
		if metadata, ok := value.(*PoolItemMetadata); ok && p.ShouldEvict(key.(string), metadata) {
			// Evict jika kebijakan terpenuhi
			pm.cache.Delete(key)
			pm.itemMetadata.Delete(key)
			pm.logger.Printf("Evicted item from pool: %s, Key: %s, LastUsed: %s", poolType, key, metadata.LastUsed)
		}
		return true
	})
}

// SmartEvictionPolicy menggabungkan kebijakan eviksi berbasis TTL, LRU, dan LFU
// Kebijakan ini memungkinkan eviksi objek berdasarkan tiga parameter: batas waktu hidup
// (TTL), waktu idle maksimum (MaxIdleTime), dan frekuensi minimum penggunaan (MinFrequency).
type SmartEvictionPolicy struct {
	TTL          time.Duration // Batas waktu TTL untuk eviksi
	MaxIdleTime  time.Duration // Batas waktu idle untuk LRU
	MinFrequency int           // Batas frekuensi untuk LFU
}

// ShouldEvict mengevaluasi apakah objek harus dieviksikan berdasarkan kombinasi kebijakan
// key: kunci unik dari objek yang dievaluasi
// metadata: metadata objek yang digunakan untuk evaluasi
// Mengembalikan nilai true jika salah satu dari ketentuan berikut terpenuhi:
// - Waktu sejak penggunaan terakhir melebihi TTL
// - Waktu idle melebihi MaxIdleTime
// - Frekuensi penggunaan kurang dari MinFrequency
func (p *SmartEvictionPolicy) ShouldEvict(key string, metadata *PoolItemMetadata) bool {
	// Jika key memiliki awalan "keep-", jangan evict objek tersebut
	if len(key) >= 5 && key[:5] == "keep-" {
		return false
	}

	// Logika eviksi berdasarkan kebijakan TTL, MaxIdleTime, atau MinFrequency
	return (p.TTL > 0 && time.Since(metadata.LastUsed) > p.TTL) ||
		(p.MaxIdleTime > 0 && time.Since(metadata.LastUsed) > p.MaxIdleTime) ||
		(p.MinFrequency > 0 && metadata.Frequency < p.MinFrequency)
}

// TTLEvictionPolicy mengimplementasikan kebijakan eviksi berdasarkan TTL
// Kebijakan ini akan menghapus objek yang sudah tidak digunakan dalam jangka waktu tertentu.
type TTLEvictionPolicy struct {
	TTL time.Duration // Batas waktu TTL untuk objek yang dieviksikan
}

// Evict mengevaluasi apakah objek harus dieviksikan
// poolType: tipe pool dari mana item akan dihapus
// Fungsi ini mencari item dengan TTL terakhir digunakan paling lama dan menghapusnya dari cache dan metadata.
func (p *TTLEvictionPolicy) Evict(poolType string, pm *PoolManager) {
	pm.itemMetadata.Range(func(key, value interface{}) bool {
		// Evaluasi kebijakan eviksi
		if metadata, ok := value.(*PoolItemMetadata); ok && p.ShouldEvict(key.(string), metadata) {
			// Hapus item dari cache dan metadata jika kebijakan eviksi terpenuhi
			pm.cache.Delete(key)
			pm.itemMetadata.Delete(key)

			// Tambahkan log dengan menggunakan key dan poolType
			pm.logger.Printf("Evicted item from pool: %s, Key: %s, LastUsed: %s, Frequency: %d",
				poolType, key, metadata.LastUsed, metadata.Frequency)
		}
		return true
	})
}

// ShouldEvict mengevaluasi apakah objek harus dieviksikan berdasarkan TTL
// key: kunci unik dari objek yang dievaluasi
// metadata: metadata objek yang digunakan untuk evaluasi
// Mengembalikan nilai true jika waktu sejak penggunaan terakhir melebihi batas TTL.
func (p *TTLEvictionPolicy) ShouldEvict(key string, metadata *PoolItemMetadata) bool {
	return time.Since(metadata.LastUsed) > p.TTL
}

// LRUEvictionPolicy mengimplementasikan kebijakan eviksi Least Recently Used (LRU)
// Kebijakan ini akan menghapus objek yang sudah tidak digunakan dalam jangka waktu tertentu.
type LRUEvictionPolicy struct {
	MaxIdleTime time.Duration // Batas waktu idle untuk objek
}

func (p *LRUEvictionPolicy) Evict(poolType string, pm *PoolManager) {
	// Tidak ada item yang dieviksikan
}

// ShouldEvict mengevaluasi apakah objek harus dieviksikan berdasarkan waktu terakhir digunakan
// key: kunci unik dari objek yang dievaluasi
// metadata: metadata objek yang digunakan untuk evaluasi
// Mengembalikan nilai true jika waktu idle sejak penggunaan terakhir melebihi MaxIdleTime.
func (p *LRUEvictionPolicy) ShouldEvict(key string, metadata *PoolItemMetadata) bool {
	return time.Since(metadata.LastUsed) > p.MaxIdleTime
}

// LFUEvictionPolicy mengimplementasikan kebijakan eviksi Least Frequently Used (LFU)
// Kebijakan ini akan menghapus objek yang jarang digunakan.
type LFUEvictionPolicy struct {
	MinFrequency int // Batas minimum frekuensi penggunaan untuk mempertahankan objek
}

// ShouldEvict mengevaluasi apakah objek harus dieviksikan berdasarkan frekuensi penggunaan
// key: kunci unik dari objek yang dievaluasi
// metadata: metadata objek yang digunakan untuk evaluasi
// Mengembalikan nilai true jika frekuensi penggunaan objek kurang dari MinFrequency.
func (p *LFUEvictionPolicy) ShouldEvict(key string, metadata *PoolItemMetadata) bool {
	return metadata.Frequency < p.MinFrequency
}
