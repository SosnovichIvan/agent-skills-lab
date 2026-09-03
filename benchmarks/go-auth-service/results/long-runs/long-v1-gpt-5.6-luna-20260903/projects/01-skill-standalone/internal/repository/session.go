package repository

import (
	"bytes"
	"sync"

	"benchmark.local/iam/internal/domain"
)

// SessionRepository is a concurrency-safe in-memory store. The token index
// contains SHA-256 digests only; the clear refresh token is never retained.
type SessionRepository struct {
	mu       sync.RWMutex
	sessions map[domain.SessionID]domain.Session
	byToken  map[[32]byte]domain.SessionID
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		sessions: make(map[domain.SessionID]domain.Session),
		byToken:  make(map[[32]byte]domain.SessionID),
	}
}

// Create stores a session and hashes refreshToken before it enters the
// repository. A session ID, user, family, and expiry are required.
func (r *SessionRepository) Create(session domain.Session, refreshToken string) error {
	if session.ID == "" || session.UserID == "" || session.TokenFamilyID == "" ||
		session.ExpiresAt.IsZero() ||
		!domain.ValidRefreshToken(refreshToken) {
		return domain.NewInvalid("invalid session")
	}
	digest := domain.HashRefreshToken(refreshToken)
	var key [32]byte
	copy(key[:], digest)
	session.RefreshTokenHash = digest
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[session.ID]; exists {
		return domain.NewConflict("session already exists")
	}
	if _, exists := r.byToken[key]; exists {
		return domain.NewConflict("refresh token already exists")
	}
	r.sessions[session.ID] = cloneSession(session)
	r.byToken[key] = session.ID
	return nil
}

// CreateSession is a descriptive alias for Create.
func (r *SessionRepository) CreateSession(session domain.Session, refreshToken string) error {
	return r.Create(session, refreshToken)
}

// GetByID returns a detached session, including revoked sessions so callers
// can distinguish rotation replay from an unknown token.
func (r *SessionRepository) GetByID(id domain.SessionID) (domain.Session, error) {
	r.mu.RLock()
	session, ok := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	return cloneSession(session), nil
}

// GetByRefreshToken finds a session by hashing the presented opaque value.
func (r *SessionRepository) GetByRefreshToken(refreshToken string) (domain.Session, error) {
	if !domain.ValidRefreshToken(refreshToken) {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	digest := domain.HashRefreshToken(refreshToken)
	var key [32]byte
	copy(key[:], digest)
	r.mu.RLock()
	id, ok := r.byToken[key]
	session := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	return cloneSession(session), nil
}

// GetByToken is a concise alias for GetByRefreshToken.
func (r *SessionRepository) GetByToken(refreshToken string) (domain.Session, error) {
	return r.GetByRefreshToken(refreshToken)
}

func (r *SessionRepository) Revoke(id domain.SessionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok {
		return domain.NewNotFound("session not found")
	}
	session.Revoked = true
	r.sessions[id] = session
	return nil
}

// RevokeFamily revokes every session belonging to the token family.
func (r *SessionRepository) RevokeFamily(family domain.TokenFamilyID) int {
	if family == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, session := range r.sessions {
		if session.TokenFamilyID == family && !session.Revoked {
			session.Revoked = true
			r.sessions[id] = session
			count++
		}
	}
	return count
}

// RevokeByUser revokes all active sessions owned by userID and returns the
// number of sessions changed. It is deliberately atomic with respect to
// session creation and refresh rotation.
func (r *SessionRepository) RevokeByUser(userID domain.UserID) int {
	if userID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, session := range r.sessions {
		if session.UserID == userID && !session.Revoked {
			session.Revoked = true
			r.sessions[id] = session
			count++
		}
	}
	return count
}

// Rotate atomically replaces the presented session with a new session in the
// same token family. The old token remains indexed so replay can revoke the
// entire family.
func (r *SessionRepository) Rotate(oldToken string, replacement domain.Session, newToken string) (domain.Session, error) {
	if !domain.ValidRefreshToken(oldToken) || !domain.ValidRefreshToken(newToken) || replacement.ID == "" || replacement.UserID == "" || replacement.TokenFamilyID == "" || replacement.ExpiresAt.IsZero() {
		return domain.Session{}, domain.NewInvalid("invalid session rotation")
	}
	oldDigest := domain.HashRefreshToken(oldToken)
	var oldKey [32]byte
	copy(oldKey[:], oldDigest)
	newDigest := domain.HashRefreshToken(newToken)
	var newKey [32]byte
	copy(newKey[:], newDigest)
	r.mu.Lock()
	defer r.mu.Unlock()
	oldID, ok := r.byToken[oldKey]
	if !ok {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	old, ok := r.sessions[oldID]
	if !ok {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	// 4.3:reuse detection — a rotated token is retained in the index so that
	// presenting it again revokes every member of its family, including tokens
	// issued after the original rotation.
	if old.Revoked {
		for id, session := range r.sessions {
			if session.TokenFamilyID == old.TokenFamilyID {
				session.Revoked = true
				r.sessions[id] = session
			}
		}
		return domain.Session{}, domain.NewUnauthorized("refresh token already used")
	}
	if _, exists := r.sessions[replacement.ID]; exists {
		return domain.Session{}, domain.NewConflict("session already exists")
	}
	if _, exists := r.byToken[newKey]; exists {
		return domain.Session{}, domain.NewConflict("refresh token already exists")
	}
	if replacement.UserID != old.UserID || replacement.TokenFamilyID != old.TokenFamilyID {
		return domain.Session{}, domain.NewInvalid("invalid session rotation")
	}
	old.Revoked = true
	r.sessions[oldID] = old
	replacement.RefreshTokenHash = newDigest
	r.sessions[replacement.ID] = cloneSession(replacement)
	r.byToken[newKey] = replacement.ID
	return cloneSession(replacement), nil
}

func (r *SessionRepository) ListByUser(userID domain.UserID) []domain.Session {
	r.mu.RLock()
	result := make([]domain.Session, 0)
	for _, session := range r.sessions {
		if session.UserID == userID {
			result = append(result, cloneSession(session))
		}
	}
	r.mu.RUnlock()
	return result
}

func cloneSession(session domain.Session) domain.Session {
	session.RefreshTokenHash = bytes.Clone(session.RefreshTokenHash)
	return session
}
