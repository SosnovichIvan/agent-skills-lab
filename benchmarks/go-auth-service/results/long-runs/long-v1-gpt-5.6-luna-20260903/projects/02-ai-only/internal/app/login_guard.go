package app

import (
	"sync"
	"time"
)

type loginAttempt struct {
	// failedLoginCounter is keyed by normalized login identity.
	failedLoginCounter int
	lockedTil          time.Time
}

type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginGuard() *loginGuard {
	return &loginGuard{attempts: make(map[string]loginAttempt)}
}

func (g *loginGuard) locked(identity string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	attempt, exists := g.attempts[identity]
	if !exists || !now.Before(attempt.lockedTil) {
		if exists && !attempt.lockedTil.IsZero() {
			delete(g.attempts, identity)
		}
		return false
	}
	return true
}

func (g *loginGuard) recordFailedLogin(identity string, now time.Time, threshold int, duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	attempt := g.attempts[identity]
	attempt.failedLoginCounter++
	if attempt.failedLoginCounter >= threshold {
		attempt.lockedTil = now.Add(duration)
	}
	g.attempts[identity] = attempt
}

func (g *loginGuard) resetFailedLoginCounter(identity string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, identity)
}
