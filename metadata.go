package poolmanager

import "time"

// PoolItemMetadata menyimpan informasi yang digunakan untuk kebijakan eviksi
// Metadata ini mencakup berbagai atribut yang membantu menentukan kapan item
// di dalam pool harus dieviksikan atau dianggap tidak lagi aktif.
type PoolItemMetadata struct {
	LastUsed       time.Time     // Terakhir kali item digunakan
	Frequency      int           // Frekuensi penggunaan item
	CreationTime   time.Time     // Waktu pembuatan item
	ExpirationTime *time.Time    // Waktu kadaluarsa item (opsional)
	UsageDuration  time.Duration // Total durasi penggunaan item
	Status         string        // Status item (misalnya, "Active", "Idle", "Evicted")
}

// UpdateItemMetadata memperbarui metadata item saat diakses
// key: kunci unik yang mengidentifikasi item dalam metadata map
// Fungsi ini memperbarui informasi seperti waktu terakhir digunakan, frekuensi
// penggunaan, dan durasi penggunaan berdasarkan waktu yang telah berlalu sejak
// terakhir kali item digunakan.
func (pm *PoolManager) UpdateItemMetadata(key string) {
	pm.safelyUpdateMetadata(key, func(itemMeta *PoolItemMetadata) {
		// Jika status item tidak aktif, tidak perlu memperbarui metadata lebih jauh
		if itemMeta.Status == "Evicted" {
			return
		}

		// Hitung waktu yang berlalu sejak terakhir kali item digunakan
		elapsed := time.Since(itemMeta.LastUsed)
		itemMeta.UsageDuration += elapsed

		// Perbarui informasi metadata
		itemMeta.LastUsed = time.Now()
		itemMeta.Frequency++
		itemMeta.Status = "Active"
	})
}

// safelyUpdateMetadata memperbarui metadata item secara aman menggunakan fungsi pembaruan yang diberikan
// key: kunci unik yang mengidentifikasi item dalam metadata map
// updateFunc: fungsi yang mendefinisikan bagaimana metadata harus diperbarui
// Fungsi ini memastikan bahwa metadata selalu diperbarui dengan cara yang aman
// menggunakan fungsi yang diberikan untuk memodifikasi metadata.
func (pm *PoolManager) safelyUpdateMetadata(key string, updateFunc func(*PoolItemMetadata)) {
	// Ambil metadata item, atau buat metadata baru jika belum ada
	metadataVal, _ := pm.itemMetadata.LoadOrStore(key, &PoolItemMetadata{
		CreationTime: time.Now(),
		LastUsed:     time.Now(),
		Status:       "Active",
	})
	metadata, ok := metadataVal.(*PoolItemMetadata)
	// Pastikan metadata ditemukan
	if !ok {
		return
	}

	// Terapkan fungsi pembaruan yang diberikan pada metadata
	updateFunc(metadata)
	pm.itemMetadata.Store(key, metadata)
}

// GetItemMetadata mengambil metadata item jika tersedia
// key: kunci unik yang mengidentifikasi item dalam metadata map
// Mengembalikan metadata item dan boolean yang menunjukkan apakah metadata ditemukan.
func (pm *PoolManager) GetItemMetadata(key string) (*PoolItemMetadata, bool) {
	metadata, ok := pm.itemMetadata.Load(key)
	if !ok {
		return nil, false
	}
	return metadata.(*PoolItemMetadata), true
}
