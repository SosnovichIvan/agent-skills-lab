package domain

import "time"

// Clock isolates domain code from the process-wide clock.
type Clock interface {
	Now() time.Time
}

// RealClock is the production clock implementation.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
