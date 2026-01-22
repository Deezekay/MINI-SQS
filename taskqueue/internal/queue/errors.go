package queue

import "errors"

var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrWorkerMismatch = errors.New("worker ID mismatch")
	ErrNotPending     = errors.New("task is not pending")
	ErrNotInFlight    = errors.New("task not in flight")
)
