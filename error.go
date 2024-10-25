package poolmanager

import "strings"

// Error constants untuk berbagai jenis kesalahan pada PoolManager
// Konstanta ini digunakan sebagai pesan dasar untuk error yang mungkin terjadi
// dalam pengelolaan pool, termasuk kesalahan saat pool tidak ditemukan atau tidak valid.
const (
	ErrPoolDoesNotExist          = "pool does not exist: "           // Error untuk pool yang tidak ditemukan
	ErrInvalidShardedPoolName    = "pool is not sharded as expected" // Error untuk tipe pool yang tidak sesuai dengan sharding
	ErrInvalidNonShardedPoolName = "pool is not a valid sync.Pool"   // Error untuk pool yang bukan tipe sync.Pool
	ErrInvalidPoolConfigType     = "invalid pool config type"        // Error untuk konfigurasi pool yang tidak valid
	ErrInvalidFactoryType        = "invalid factory type"            // Error untuk tipe factory yang tidak valid
)

// PoolError adalah tipe error khusus yang digunakan untuk mencatat kesalahan pada operasi PoolManager
// PoolError menyimpan informasi tentang tipe pool, operasi yang gagal, dan error asli yang menyebabkan kegagalan.
type PoolError struct {
	PoolName  string // Tipe pool tempat kesalahan terjadi
	Operation string // Operasi yang gagal dijalankan
	Err       error  // Error asli yang menyebabkan kegagalan
}

// NewPoolError membuat instance PoolError baru dengan informasi tentang poolName, operasi, dan error yang terjadi
// poolName: tipe pool yang menyebabkan kesalahan
// operation: nama operasi yang menyebabkan kesalahan (misalnya "add", "get", atau "put")
// err: error asli yang menyebabkan kegagalan
// Fungsi ini mengembalikan pointer ke PoolError yang baru dibuat.
func NewPoolError(poolName, operation string, err error) *PoolError {
	return &PoolError{
		PoolName:  poolName,
		Operation: operation,
		Err:       err,
	}
}

// Error mengimplementasikan interface error dan mengembalikan pesan kesalahan yang lebih terperinci
// Fungsi ini membuat pesan error yang menjelaskan jenis pool, operasi yang gagal, dan error asli.
func (e *PoolError) Error() string {
	var sb strings.Builder
	sb.WriteString("pool error: ")
	sb.WriteString(e.PoolName)
	sb.WriteString(" during ")
	sb.WriteString(e.Operation)
	sb.WriteString(" operation: ")
	sb.WriteString(e.Err.Error())
	return sb.String()
}

// Unwrap mengembalikan error asli dari PoolError
// Fungsi ini memungkinkan pengguna untuk mendapatkan error yang terbungkus dalam PoolError
// dengan menggunakan fungsi errors.Unwrap().
func (e *PoolError) Unwrap() error {
	return e.Err
}
