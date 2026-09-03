package domain

import "time"

// Clock supplies the current time to domain services. Injecting it keeps
// domain behavior deterministic and independent of the global clock.
type Clock interface {
	Now() time.Time
}

// RealClock is the production clock implementation.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

// ClockFunc adapts a function to Clock and is useful for deterministic callers.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }
