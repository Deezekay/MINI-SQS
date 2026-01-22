package config

import "time"

// Config holds all tunable server parameters.
type Config struct {
	ListenAddr             string
	VisibilityTimeout      time.Duration
	MaxPayloadBytes        int
	MaxTasksInMemory       int
	WALFilePath            string
	WALSyncOnWrite         bool
	VisibilityScanInterval time.Duration
}
