package domain

import "time"

// Clock supplies time to domain logic. Passing it as a dependency keeps
// expiration and timestamp behavior deterministic without global clock calls.
type Clock interface {
	Now() time.Time
}

// RealClock is the production clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
