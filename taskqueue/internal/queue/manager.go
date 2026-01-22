package queue

import (
	"fmt"
	"sync"
	"taskqueue/internal/clock"
	"taskqueue/internal/config"
	"taskqueue/internal/wal"
	"time"
)

// Manager owns all in-memory state.
type Manager struct {
	mu sync.Mutex

	tasks    map[string]*Task
	pending  []string
	inFlight map[string]string // taskID → workerID

	wal    *wal.WAL
	clock  clock.Clock
	config config.Config
}

// NewManager initializes an empty manager.
// WAL is attached later via replay or setter to adhere to strict init order.
func NewManager(cfg config.Config, clk clock.Clock) *Manager {
	return &Manager{
		tasks:    make(map[string]*Task),
		pending:  make([]string, 0),
		inFlight: make(map[string]string),
		config:   cfg,
		clock:    clk,
	}
}

// SetWAL attaches the WAL to the manager.
func (m *Manager) SetWAL(w *wal.WAL) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wal = w
}

func (m *Manager) ReplayEnqueue(taskID, payload string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; exists {
		return
	}

	task := &Task{
		ID:       taskID,
		Payload:  payload,
		State:    Pending,
		Attempts: 0,
	}
	m.tasks[taskID] = task
	m.pending = append(m.pending, taskID)
}

func (m *Manager) ReplayPoll(taskID, workerID string, deadline time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return
	}

	newPending := make([]string, 0, len(m.pending))
	found := false
	for _, id := range m.pending {
		if id == taskID && !found {
			found = true
			continue
		}
		newPending = append(newPending, id)
	}
	m.pending = newPending

	task.State = InFlight
	task.VisibilityDeadline = deadline
	m.inFlight[taskID] = workerID
}

func (m *Manager) ReplayAck(taskID, workerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return
	}

	task.State = Done
	delete(m.inFlight, taskID)
}

func (m *Manager) ReplayTimeout(taskID string, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return
	}

	if task.State == InFlight {
		task.Attempts++
		task.State = Pending
		delete(m.inFlight, taskID)
		m.pending = append(m.pending, taskID)
	}
}

// Public API methods

// Enqueue adds a task to the queue.
func (m *Manager) Enqueue(taskID, payload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validation
	if len(payload) > m.config.MaxPayloadBytes {
		return fmt.Errorf("payload exceeds max size")
	}
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	// Idempotency: If task_id exists -> return success
	if _, exists := m.tasks[taskID]; exists {
		return nil
	}

	// Capacity check
	if len(m.tasks) >= m.config.MaxTasksInMemory {
		return fmt.Errorf("max tasks capacity reached")
	}

	// Write WAL
	rec := wal.Record{
		Type:      wal.EnqueueRecord,
		TaskID:    taskID,
		Payload:   payload,
		Timestamp: m.clock.Now(),
	}
	if err := m.wal.WriteRecord(rec); err != nil {
		return fmt.Errorf("wal write failed: %w", err)
	}

	// Mutate Memory
	task := &Task{
		ID:       taskID,
		Payload:  payload,
		State:    Pending,
		Attempts: 0,
	}
	m.tasks[taskID] = task
	m.pending = append(m.pending, taskID)

	return nil
}

// Poll retrieves a pending task.
func (m *Manager) Poll(workerID string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if workerID == "" {
		return nil, fmt.Errorf("worker ID required")
	}

	// Loop to find valid task
	for len(m.pending) > 0 {
		// Pop oldest
		taskID := m.pending[0]
		m.pending = m.pending[1:]

		task, exists := m.tasks[taskID]
		if !exists || task.State != Pending {
			continue
		}

		deadline := m.clock.Now().Add(m.config.VisibilityTimeout)

		rec := wal.Record{
			Type:      wal.PollRecord,
			TaskID:    taskID,
			WorkerID:  workerID,
			Deadline:  deadline,
			Timestamp: m.clock.Now(),
		}
		if err := m.wal.WriteRecord(rec); err != nil {
			// Restore pending if WAL fails
			m.pending = append([]string{taskID}, m.pending...)
			return nil, fmt.Errorf("wal write failed: %w", err)
		}

		task.State = InFlight
		task.VisibilityDeadline = deadline
		m.inFlight[taskID] = workerID

		return task, nil
	}

	return nil, nil // No content
}

// Ack acknowledges a task.
func (m *Manager) Ack(taskID, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found")
	}

	if task.State == Done {
		return nil // Idempotent success
	}

	if task.State != InFlight {
		return fmt.Errorf("task not in flight")
	}

	if m.inFlight[taskID] != workerID {
		return fmt.Errorf("worker ID mismatch")
	}

	// Write WAL
	rec := wal.Record{
		Type:      wal.AckRecord,
		TaskID:    taskID,
		WorkerID:  workerID,
		Timestamp: m.clock.Now(),
	}
	if err := m.wal.WriteRecord(rec); err != nil {
		return fmt.Errorf("wal write failed: %w", err)
	}

	// Mutate Memory
	task.State = Done
	delete(m.inFlight, taskID)

	return nil
}

// ProcessTimeouts checks for expired visibility deadlines.
func (m *Manager) ProcessTimeouts() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()

	for taskID, workerID := range m.inFlight {
		task, exists := m.tasks[taskID]
		if !exists {
			delete(m.inFlight, taskID)
			continue
		}

		if now.After(task.VisibilityDeadline) {
			rec := wal.Record{
				Type:      wal.TimeoutRecord,
				TaskID:    taskID,
				WorkerID:  workerID,
				Timestamp: now,
			}
			if err := m.wal.WriteRecord(rec); err != nil {
				return fmt.Errorf("wal write failed for task %s: %w", taskID, err)
			}
		}
	}
	return nil
}

// ScanVisibility checks one expired task at a time or iterates all?
// Step 11 says: "For each IN_FLIGHT task: If clock.Now() > VisibilityDeadline ... "
// We can just iterate the map.
func (m *Manager) ScanVisibility() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock.Now()

	// Collect candidates first to avoid map modification during iteration
	var candidates []string
	for taskID := range m.inFlight {
		candidates = append(candidates, taskID)
	}

	for _, taskID := range candidates {
		task, exists := m.tasks[taskID]
		if !exists {
			continue
		}

		if now.After(task.VisibilityDeadline) {
			rec := wal.Record{
				Type:      wal.TimeoutRecord,
				TaskID:    taskID,
				Timestamp: now,
			}
			if err := m.wal.WriteRecord(rec); err != nil {
				fmt.Printf("Error writing timeout WAL for task %s: %v\n", taskID, err)
				continue
			}

			task.Attempts++
			task.State = Pending
			delete(m.inFlight, taskID)
			m.pending = append(m.pending, taskID)
		}
	}
}

// GetStats returns metrics for the metrics endpoint.
func (m *Manager) GetStats() (total, pending, inFlight, done, retries int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total = len(m.tasks)
	pending = len(m.pending)
	inFlight = len(m.inFlight)

	// Done = Total - Pending - InFlight (roughly, if no other states)
	// Or we just count.
	done = 0
	retries = 0
	for _, t := range m.tasks {
		if t.State == Done {
			done++
		}
		retries += t.Attempts
	}
	return
}
