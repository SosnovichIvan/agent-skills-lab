package security

import (
	"errors"
	"sync"
	"time"
)

type loginFailureState struct {
	failedLoginCounter int
	lockedUntil        time.Time
}

// LoginGuard tracks authentication failures by normalized identity.
type LoginGuard struct {
	mu           sync.Mutex
	maxFailures  int
	lockDuration time.Duration
	states       map[string]loginFailureState
}

func NewLoginGuard(maxFailures int, lockDuration time.Duration) (*LoginGuard, error) {
	if maxFailures <= 0 {
		return nil, errors.New("login max failures must be positive")
	}
	if lockDuration <= 0 {
		return nil, errors.New("login lock duration must be positive")
	}
	return &LoginGuard{maxFailures: maxFailures, lockDuration: lockDuration, states: make(map[string]loginFailureState)}, nil
}

func (g *LoginGuard) Locked(identity string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	state, ok := g.states[identity]
	if !ok {
		return false
	}
	if !state.lockedUntil.IsZero() && now.Before(state.lockedUntil) {
		return true
	}
	if !state.lockedUntil.IsZero() {
		delete(g.states, identity)
	}
	return false
}

func (g *LoginGuard) RecordFailure(identity string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.states[identity]
	if !state.lockedUntil.IsZero() && now.Before(state.lockedUntil) {
		return
	}
	if !state.lockedUntil.IsZero() {
		state = loginFailureState{}
	}
	// 5.4:failed login counter
	state.failedLoginCounter++
	if state.failedLoginCounter >= g.maxFailures {
		state.lockedUntil = now.Add(g.lockDuration)
	}
	g.states[identity] = state
}

func (g *LoginGuard) RecordSuccess(identity string) {
	g.mu.Lock()
	delete(g.states, identity)
	g.mu.Unlock()
}
