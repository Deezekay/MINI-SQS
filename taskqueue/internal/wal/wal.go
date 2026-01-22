package wal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// WAL handles writing records to disk.
type WAL struct {
	mu          sync.Mutex
	file        *os.File
	syncOnWrite bool
}

// NewWAL creates or opens a WAL file.
func NewWAL(path string, syncOnWrite bool) (*WAL, error) {
	// Security: 0600 is standard for private data files.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		file:        file,
		syncOnWrite: syncOnWrite,
	}, nil
}

// WriteRecord appends a record to the WAL.
func (w *WAL) WriteRecord(rec Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Append newline to make it one JSON object per line
	data = append(data, '\n')

	// Independent Write call
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("failed to write WAL record: %w", err)
	}

	if w.syncOnWrite {
		if err := w.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL file: %w", err)
		}
	}

	return nil
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
