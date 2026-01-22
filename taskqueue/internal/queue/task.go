package queue

import "time"

// TaskState defines the state of a task.
type TaskState string

const (
	Pending  TaskState = "PENDING"
	InFlight TaskState = "IN_FLIGHT"
	Done     TaskState = "DONE"
)

// Task represents a unit of work.
type Task struct {
	ID                 string
	Payload            string
	State              TaskState
	VisibilityDeadline time.Time
	Attempts           int
}
