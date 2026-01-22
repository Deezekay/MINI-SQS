package queue

import (
	"time"
)

// StartVisibilityScanner calls ProcessTimeouts in a background loop.
func (m *Manager) StartVisibilityScanner() {
	go func() {
		ticker := time.NewTicker(m.config.VisibilityScanInterval)
		defer ticker.Stop()

		for range ticker.C {
			m.ScanVisibility()
		}
	}()
}
