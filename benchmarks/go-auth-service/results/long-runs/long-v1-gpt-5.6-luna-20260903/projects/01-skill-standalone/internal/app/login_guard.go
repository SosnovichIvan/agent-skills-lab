package app

import (
	"sync"
	"time"
)

// loginGuard tracks credential failures by the normalized login identity.
// State is process-local because this application currently uses in-memory
// repositories; the mutex makes the decision and update atomic per process.
type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{attempts: make(map[string]loginAttempt)}
}

func (g *loginGuard) isLocked(identity string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	attempt, exists := g.attempts[identity]
	if !exists {
		return false
	}
	if !attempt.lockedUntil.IsZero() && now.Before(attempt.lockedUntil) {
		return true
	}
	if !attempt.lockedUntil.IsZero() {
		delete(g.attempts, identity)
	}
	return false
}

func (g *loginGuard) failure(identity string, now time.Time, threshold int, duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	attempt := g.attempts[identity]
	if !attempt.lockedUntil.IsZero() && !now.Before(attempt.lockedUntil) {
		attempt = loginAttempt{}
	}
	attempt.failures++
	if attempt.failures >= threshold {
		attempt.lockedUntil = now.Add(duration)
	}
	g.attempts[identity] = attempt
}

func (g *loginGuard) success(identity string) {
	g.mu.Lock()
	delete(g.attempts, identity)
	g.mu.Unlock()
}
