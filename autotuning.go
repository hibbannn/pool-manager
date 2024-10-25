package poolmanager

func (pm *PoolManager) autoTunePoolSize() {
	pm.pools.Range(func(key, value interface{}) bool {
		poolName, ok := key.(string)
		if !ok {
			return true
		}

		configVal, ok := pm.poolConfig.Load(poolName)
		if !ok {
			return true
		}

		conf, ok := configVal.(PoolConfiguration)
		if !ok || !conf.AutoTune {
			return true
		}

		// Hitung ukuran pool saat ini
		currentSize := pm.getCurrentPoolSize(poolName, value)
		if currentSize == 0 {
			pm.logger.Printf("Skipping auto-tuning for empty pool: %s", poolName)
			return true
		}

		// Tentukan ukuran pool baru berdasarkan faktor auto-tuning
		var factor float64
		if conf.AutoTuneDynamicFactor != nil {
			factor = conf.AutoTuneDynamicFactor(currentSize)
		} else {
			factor = conf.AutoTuneFactor
		}

		// Hitung ukuran pool baru dan batasi sesuai konfigurasi
		newSize := int(float64(currentSize) * factor)
		if newSize > conf.MaxSize {
			newSize = conf.MaxSize
		} else if newSize < conf.MinSize {
			newSize = conf.MinSize
		}

		// Hanya ubah ukuran pool jika berbeda dari ukuran saat ini
		if newSize != currentSize {
			pm.ResizePool(poolName, newSize)
			pm.logger.Printf("Auto-tuned pool %s from %d to new size: %d", poolName, currentSize, newSize)
			if conf.OnAutoTune != nil {
				conf.OnAutoTune(poolName, newSize)
			}
		}

		return true
	})
}
