package poolmanager

// LogLevel mendefinisikan tingkat log yang didukung
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarningLevel
	ErrorLevel
)

// SetLogLevel mengatur tingkat log untuk PoolManager
func (pm *PoolManager) SetLogLevel(level LogLevel) {
	pm.monitoringConfig.LogLevel = level
}
