package queue

import (
	"testing"
	"time"

	"os"
	"taskqueue/internal/config"
	"taskqueue/internal/wal"
)

// MockClock for deterministic testing
type MockClock struct {
	currentTime time.Time
}

func (m *MockClock) Now() time.Time {
	return m.currentTime
}

func (m *MockClock) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

func TestManager_Flow_And_Recovery(t *testing.T) {
	// Setup
	walPath := "test_flow.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	cfg := config.Config{
		ListenAddr:        ":8080",
		VisibilityTimeout: 10 * time.Second,
		MaxPayloadBytes:   1024,
		MaxTasksInMemory:  100,
		WALFilePath:       walPath,
		WALSyncOnWrite:    true,
	}
	clk := &MockClock{currentTime: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)}

	// 1. Initialize Manager 1
	mgr := NewManager(cfg, clk)
	w, err := wal.NewWAL(walPath, true)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	mgr.SetWAL(w)

	// 2. Enqueue Task 1
	taskID := "task-1"
	if err := mgr.Enqueue(taskID, "payload-1"); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// 3. Poll Task
	workerID := "worker-1"
	task, err := mgr.Poll(workerID)
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}
	if task == nil {
		t.Fatalf("Expected task, got nil")
	}
	if task.ID != taskID {
		t.Errorf("Expected ID %s, got %s", taskID, task.ID)
	}

	// 4. Simulate Crash (Close WAL, create new Manager)
	w.Close()

	// New Manager
	mgr2 := NewManager(cfg, clk)
	// Replay
	if err := wal.Replay(walPath, mgr2); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	// Attach new WAL (append mode)
	w2, err := wal.NewWAL(walPath, true)
	if err != nil {
		t.Fatalf("Failed to open WAL 2: %v", err)
	}
	mgr2.SetWAL(w2)
	defer w2.Close()

	// 5. Verify State after recovery
	// Task should be InFlight because we haven't acked or timed out
	// Wait, Poll moves to InFlight.
	// But in-memory state needs to be restored.
	// Manager.inFlight should have the task.

	// Check internal state (whitebox testing allowed since this is part of the package)
	if _, exists := mgr2.inFlight[taskID]; !exists {
		t.Errorf("Task %s should be in-flight after recovery", taskID)
	}

	// 6. Simulate Visibility Timeout
	clk.Advance(11 * time.Second) // > 10s timeout
	mgr2.ScanVisibility()

	// 7. Verify Task is Pending again
	if _, exists := mgr2.inFlight[taskID]; exists {
		t.Errorf("Task %s should NOT be in-flight after timeout", taskID)
	}
	// Should be in pending?
	// We can try to Poll it again
	task2, err := mgr2.Poll("worker-2")
	if err != nil {
		t.Fatalf("Poll 2 failed: %v", err)
	}
	if task2 == nil {
		t.Fatalf("Expected task after timeout, got nil")
	}
	if task2.ID != taskID {
		t.Errorf("Expected ID %s, got %s", taskID, task2.ID)
	}
	if task2.Attempts != 1 {
		// Attempts is 0 on initial enqueue, attempts increment on timeout.
		// Wait, Step 11: "Increment Attempts".
		// We expect 1 attempt now. (0 -> InFlight -> Timeout -> 1)
		t.Errorf("Expected attempts 1, got %d", task2.Attempts)
	}

	// 8. Ack
	if err := mgr2.Ack(taskID, "worker-2"); err != nil {
		t.Fatalf("Ack failed: %v", err)
	}

	// 9. Verify Done
	if task2.State != Done {
		t.Errorf("Expected Done, got %s", task2.State)
	}
}

func TestManager_Idempotency(t *testing.T) {
	walPath := "test_copy.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	cfg := config.Config{WALFilePath: walPath, MaxTasksInMemory: 10, MaxPayloadBytes: 100}
	clk := &MockClock{}
	mgr := NewManager(cfg, clk)
	w, _ := wal.NewWAL(walPath, false)
	mgr.SetWAL(w)
	defer w.Close()

	// Enqueue duplicate
	if err := mgr.Enqueue("t1", "p"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Enqueue("t1", "p"); err != nil {
		t.Fatal(err)
	}

	stats, _, _, _, _ := mgr.GetStats()
	if stats != 1 {
		t.Errorf("Expected 1 task, got %d", stats)
	}
}
