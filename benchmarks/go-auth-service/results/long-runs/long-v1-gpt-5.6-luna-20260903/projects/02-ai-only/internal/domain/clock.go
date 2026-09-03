package domain

import "time"

// Clock abstracts time for domain logic and makes time-dependent behavior
// deterministic in callers that provide their own implementation.
type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
