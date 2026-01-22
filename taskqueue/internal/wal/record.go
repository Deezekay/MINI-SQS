package wal

import "time"

// RecordType defines exact record types.
type RecordType string

const (
	EnqueueRecord RecordType = "ENQUEUE"
	PollRecord    RecordType = "POLL"
	AckRecord     RecordType = "ACK"
	TimeoutRecord RecordType = "TIMEOUT"
)

// Record represents a single event in the WAL.
type Record struct {
	Type      RecordType `json:"type"`
	TaskID    string     `json:"task_id"`
	Payload   string     `json:"payload,omitempty"`
	Deadline  time.Time  `json:"deadline,omitempty"`
	WorkerID  string     `json:"worker_id,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}
