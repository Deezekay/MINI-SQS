package clock

import "time"

// Clock interface allows mocking time.Now() for deterministic testing.
type Clock interface {
	Now() time.Time
}

// RealClock wraps time.Now()
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
