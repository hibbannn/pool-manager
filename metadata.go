package poolmanager

import "time"

const (
	NoEvictionPolicy      = "no_eviction"
	DefaultEvictionPolicy = "default_eviction"
)

// PoolItemMetadata menyimpan informasi yang digunakan untuk kebijakan eviksi
// Metadata ini mencakup berbagai atribut yang membantu menentukan kapan item
// di dalam pool harus dieviksikan atau dianggap tidak lagi aktif.
type PoolItemMetadata struct {
	PoolName         string            // Nama pool yang mengelola item
	LastUsed         time.Time         // Terakhir kali item digunakan
	Frequency        int               // Frekuensi penggunaan item
	CreationTime     time.Time         // Waktu pembuatan item
	ExpirationTime   *time.Time        // Waktu kadaluarsa item (opsional)
	UsageDuration    time.Duration     // Total durasi penggunaan item
	Status           string            // Status item (misalnya, "Active", "Idle", "Evicted")
	OwnerID          string            // ID pemilik saat ini (opsional)
	AccessCount      int               // Jumlah total akses (penggunaan) item
	IdleDuration     time.Duration     // Durasi waktu item idle
	MaxUsageDuration time.Duration     // Batas maksimal waktu penggunaan
	IsPooled         bool              // Apakah item sedang berada di pool atau sedang digunakan
	Tag              map[string]string // Tag untuk penyimpanan informasi tambahan
	LastResetTime    time.Time         // Waktu terakhir item di-reset
}
