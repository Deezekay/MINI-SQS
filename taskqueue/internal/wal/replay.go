package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ReplayTarget defines the methods to restore state.
type ReplayTarget interface {
	ReplayEnqueue(taskID, payload string)
	ReplayPoll(taskID, workerID string, deadline time.Time)
	ReplayAck(taskID, workerID string)
	ReplayTimeout(taskID string, now time.Time)
}

// Replay reads the WAL sequence and reapplies state.
func Replay(walPath string, target ReplayTarget) error {
	file, err := os.Open(walPath)
	if os.IsNotExist(err) {
		return nil // No WAL yet, valid empty state
	}
	if err != nil {
		return fmt.Errorf("failed to open WAL for replay: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("corrupt WAL at line %d: %w", lineNum, err)
		}

		// Reapply state
		switch rec.Type {
		case EnqueueRecord:
			target.ReplayEnqueue(rec.TaskID, rec.Payload)
		case PollRecord:
			target.ReplayPoll(rec.TaskID, rec.WorkerID, rec.Deadline)
		case AckRecord:
			target.ReplayAck(rec.TaskID, rec.WorkerID)
		case TimeoutRecord:
			target.ReplayTimeout(rec.TaskID, rec.Timestamp)
		default:
			return fmt.Errorf("unknown record type at line %d: %s", lineNum, rec.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL: %w", err)
	}

	return nil
}
