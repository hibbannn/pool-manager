package main

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	poolmanager "github.com/hibbannn/pool-manager"
)

// Matrix adalah struktur untuk menyimpan data matriks besar
type Matrix struct {
	Data [][]float64
	Rows int
	Cols int
}

// NewMatrix membuat matriks baru dengan ukuran yang ditentukan
func NewMatrix(rows, cols int) *Matrix {
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &Matrix{Data: data, Rows: rows, Cols: cols}
}

// Reset mengatur ulang matriks sebelum dikembalikan ke pool
func (m *Matrix) Reset() {
	for i := range m.Data {
		for j := range m.Data[i] {
			m.Data[i][j] = 0
		}
	}
}

// RandomFill mengisi matriks dengan nilai acak
func (m *Matrix) RandomFill() {
	for i := range m.Data {
		for j := range m.Data[i] {
			m.Data[i][j] = rand.Float64()
		}
	}
}

// Multiply melakukan perkalian dua matriks dan menghasilkan matriks baru
func (m *Matrix) Multiply(other *Matrix) *Matrix {
	if m.Cols != other.Rows {
		log.Fatalf("Ukuran matriks tidak sesuai untuk perkalian")
	}
	result := NewMatrix(m.Rows, other.Cols)
	for i := 0; i < m.Rows; i++ {
		for j := 0; j < other.Cols; j++ {
			sum := 0.0
			for k := 0; k < m.Cols; k++ {
				sum += m.Data[i][k] * other.Data[k][j]
			}
			result.Data[i][j] = sum
		}
	}
	return result
}

// MeasureMemoryUsage mengukur penggunaan memori saat ini
func MeasureMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc / 1024 // Memori yang digunakan dalam KB
}

// performMatrixOperationsWithoutPool mensimulasikan operasi matriks tanpa menggunakan pool
func performMatrixOperationsWithoutPool(numOperations int, matrixSize int) {
	start := time.Now()
	initialMemory := MeasureMemoryUsage()

	var wg sync.WaitGroup
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Membuat dua matriks baru untuk setiap operasi
			matrixA := NewMatrix(matrixSize, matrixSize)
			matrixB := NewMatrix(matrixSize, matrixSize)
			matrixA.RandomFill()
			matrixB.RandomFill()
			// Lakukan perkalian matriks
			_ = matrixA.Multiply(matrixB)
		}()
	}
	wg.Wait()

	finalMemory := MeasureMemoryUsage()
	fmt.Printf("Tanpa Pool: Waktu = %v, Memori Digunakan = %d KB\n", time.Since(start), finalMemory-initialMemory)
}

// performMatrixOperationsWithPool mensimulasikan operasi matriks dengan menggunakan pool
func performMatrixOperationsWithPool(pm *poolmanager.PoolManager, numOperations int, matrixSize int) {
	start := time.Now()
	initialMemory := MeasureMemoryUsage()

	var wg sync.WaitGroup
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Mengambil dua matriks dari pool
			instanceA, err := pm.AcquireInstance("MatrixPool")
			if err != nil {
				log.Printf("Error acquiring instance: %v", err)
				return
			}
			instanceB, err := pm.AcquireInstance("MatrixPool")
			if err != nil {
				log.Printf("Error acquiring instance: %v", err)
				return
			}

			matrixA := instanceA.(*Matrix)
			matrixB := instanceB.(*Matrix)
			matrixA.RandomFill()
			matrixB.RandomFill()

			// Lakukan perkalian matriks
			_ = matrixA.Multiply(matrixB)

			// Kembalikan matriks ke pool
			if err := pm.ReleaseInstance("MatrixPool", matrixA); err != nil {
				log.Printf("Error releasing instance: %v", err)
			}
			if err := pm.ReleaseInstance("MatrixPool", matrixB); err != nil {
				log.Printf("Error releasing instance: %v", err)
			}
		}()
	}
	wg.Wait()

	finalMemory := MeasureMemoryUsage()
	fmt.Printf("Dengan PoolManager (Auto-Tune): Waktu = %v, Memori Digunakan = %d KB\n", time.Since(start), finalMemory-initialMemory)
}

func main() {
	// Konfigurasi PoolManager
	config, err := poolmanager.NewPoolConfiguration("MatrixPool").
		WithSizeLimit(2).
		WithInitialSize(1).
		WithMinSize(1).
		WithMaxSize(5).
		Build()
	if err != nil {
		log.Fatalf("Error creating pool configuration: %v", err)
	}

	// Buat PoolManager baru
	pm := poolmanager.NewPoolManager(config)
	err = pm.AddPool("MatrixPool", func() poolmanager.PoolAble {
		return NewMatrix(100, 100) // Ukuran matriks 100x100
	}, config)
	if err != nil {
		log.Fatalf("Error adding pool: %v", err)
	}

	// Jumlah operasi dan ukuran matriks untuk simulasi
	numOperations := 10000
	matrixSize := 100

	fmt.Println("Memulai perbandingan dengan beban kerja berat...")
	performMatrixOperationsWithoutPool(numOperations, matrixSize)
	performMatrixOperationsWithPool(pm, numOperations, matrixSize)
	fmt.Println("Perbandingan selesai.")
}
