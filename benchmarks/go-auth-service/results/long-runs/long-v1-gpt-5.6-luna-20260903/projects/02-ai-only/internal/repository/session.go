package repository

import (
	"crypto/subtle"
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type SessionRepository struct {
	mu      sync.RWMutex
	byID    map[domain.SessionID]domain.Session
	byToken map[string]domain.SessionID
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		byID:    make(map[domain.SessionID]domain.Session),
		byToken: make(map[string]domain.SessionID),
	}
}

func (r *SessionRepository) Create(session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if session.ID == "" || session.UserID == "" || session.FamilyID == "" || len(session.RefreshTokenHash) == 0 {
		return domain.Invalid("session is incomplete")
	}
	if _, exists := r.byID[session.ID]; exists {
		return domain.Conflict("session already exists")
	}
	tokenKey := hex.EncodeToString(session.RefreshTokenHash)
	if _, exists := r.byToken[tokenKey]; exists {
		return domain.Conflict("refresh token already exists")
	}
	r.byID[session.ID] = cloneSession(session)
	r.byToken[tokenKey] = session.ID
	return nil
}

func (r *SessionRepository) Get(id domain.SessionID) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.byID[id]
	if !exists {
		return domain.Session{}, domain.NotFound("session not found")
	}
	return cloneSession(session), nil
}

func (r *SessionRepository) ListByUser(userID domain.UserID) []domain.Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]domain.Session, 0)
	for _, session := range r.byID {
		if session.UserID == userID {
			sessions = append(sessions, cloneSession(session))
		}
	}
	return sessions
}

func (r *SessionRepository) FindByRefreshTokenHash(hash []byte) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessionID, exists := r.byToken[hex.EncodeToString(hash)]
	if !exists {
		return domain.Session{}, domain.NotFound("session not found")
	}
	session, exists := r.byID[sessionID]
	if !exists || subtle.ConstantTimeCompare(session.RefreshTokenHash, hash) != 1 {
		return domain.Session{}, domain.NotFound("session not found")
	}
	return cloneSession(session), nil
}

func (r *SessionRepository) Revoke(id domain.SessionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.byID[id]
	if !exists {
		return domain.NotFound("session not found")
	}
	session.Revoked = true
	r.byID[id] = session
	return nil
}

func (r *SessionRepository) RevokeAllByUser(userID domain.UserID) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for id, session := range r.byID {
		if session.UserID == userID && !session.Revoked {
			session.Revoked = true
			r.byID[id] = session
			count++
		}
	}
	return count
}

// Rotate atomically revokes the session identified by oldHash and stores its
// replacement. This prevents two concurrent requests from both consuming the
// same refresh token.
func (r *SessionRepository) Rotate(oldHash []byte, replacement domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldID, exists := r.byToken[hex.EncodeToString(oldHash)]
	if !exists {
		return domain.Unauthorized("refresh token is invalid")
	}
	old, exists := r.byID[oldID]
	if !exists || subtle.ConstantTimeCompare(old.RefreshTokenHash, oldHash) != 1 {
		return domain.Unauthorized("refresh token is invalid")
	}
	if old.Revoked {
		r.revokeFamilyLocked(old.FamilyID)
		return domain.ErrTokenReuse
	}
	if replacement.ID == "" || replacement.UserID == "" || replacement.FamilyID == "" || len(replacement.RefreshTokenHash) == 0 {
		return domain.Invalid("session is incomplete")
	}
	if _, exists := r.byID[replacement.ID]; exists {
		return domain.Conflict("session already exists")
	}
	newTokenKey := hex.EncodeToString(replacement.RefreshTokenHash)
	if _, exists := r.byToken[newTokenKey]; exists {
		return domain.Conflict("refresh token already exists")
	}
	old.Revoked = true
	r.byID[oldID] = old
	r.byID[replacement.ID] = cloneSession(replacement)
	r.byToken[newTokenKey] = replacement.ID
	return nil
}

func (r *SessionRepository) RevokeFamily(familyID domain.ID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.revokeFamilyLocked(familyID)
}

func (r *SessionRepository) revokeFamilyLocked(familyID domain.ID) int {
	count := 0
	for id, session := range r.byID {
		if session.FamilyID == familyID && !session.Revoked {
			session.Revoked = true
			r.byID[id] = session
			count++
		}
	}
	return count
}

func cloneSession(session domain.Session) domain.Session {
	session.RefreshTokenHash = append([]byte(nil), session.RefreshTokenHash...)
	return session
}
