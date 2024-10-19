package poolmanager

// PoolAble adalah interface yang HARUS DIIMPLEMENTASIKAN oleh struct yang ingin menggunakan pooling.
// Interface ini menentukan bahwa struct harus memiliki metode Reset() untuk mengatur ulang
// kondisi objek sebelum dikembalikan ke pool.
type PoolAble interface {
	// Reset mengatur ulang kondisi objek ke keadaan semula sebelum dikembalikan ke pool.
	// Metode ini memungkinkan objek untuk digunakan kembali tanpa meninggalkan data sebelumnya.
	Reset()
}
