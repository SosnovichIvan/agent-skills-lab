package repository

import (
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

// SessionStore is the persistence contract for refresh-token sessions.
type SessionStore interface {
	Create(session domain.Session) error
	Get(id domain.ID) (domain.Session, error)
	FindByTokenHash(hash []byte) (domain.Session, error)
	Update(session domain.Session) error
	Rotate(oldTokenHash []byte, replacement domain.Session) error
	RevokeFamily(familyID domain.ID) error
	ListByUser(userID domain.ID) []domain.Session
	Revoke(id domain.ID) error
	RevokeAllByUser(userID domain.ID) error
}

// SessionRepository stores session metadata indexed by session ID and token
// digest. It never accepts or stores a raw refresh token.
type SessionRepository struct {
	mu      sync.RWMutex
	byID    map[domain.ID]domain.Session
	byToken map[string]domain.ID
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		byID:    make(map[domain.ID]domain.Session),
		byToken: make(map[string]domain.ID),
	}
}

func (r *SessionRepository) Create(session domain.Session) error {
	if err := validateSession(session); err != nil {
		return err
	}
	tokenKey := hex.EncodeToString(session.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[session.ID]; exists {
		return domain.NewError(domain.KindConflict, "session ID already exists")
	}
	if _, exists := r.byToken[tokenKey]; exists {
		return domain.NewError(domain.KindConflict, "session token already exists")
	}
	r.byID[session.ID] = cloneSession(session)
	r.byToken[tokenKey] = session.ID
	return nil
}

func (r *SessionRepository) Get(id domain.ID) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.byID[id]
	if !ok {
		return domain.Session{}, domain.NewError(domain.KindNotFound, "session not found")
	}
	return cloneSession(session), nil
}

func (r *SessionRepository) FindByTokenHash(hash []byte) (domain.Session, error) {
	tokenKey := hex.EncodeToString(hash)
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byToken[tokenKey]
	if !ok {
		return domain.Session{}, domain.NewError(domain.KindNotFound, "session not found")
	}
	return cloneSession(r.byID[id]), nil
}

func (r *SessionRepository) Update(session domain.Session) error {
	if err := validateSession(session); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.byID[session.ID]
	if !exists {
		return domain.NewError(domain.KindNotFound, "session not found")
	}
	newKey := hex.EncodeToString(session.TokenHash)
	if indexedID, exists := r.byToken[newKey]; exists && indexedID != session.ID {
		return domain.NewError(domain.KindConflict, "session token already exists")
	}
	delete(r.byToken, hex.EncodeToString(previous.TokenHash))
	r.byID[session.ID] = cloneSession(session)
	r.byToken[newKey] = session.ID
	return nil
}

// Rotate atomically revokes the session identified by oldTokenHash and
// inserts its replacement. This prevents two concurrent refresh requests
// from successfully consuming the same token.
func (r *SessionRepository) Rotate(oldTokenHash []byte, replacement domain.Session) error {
	if err := validateSession(replacement); err != nil {
		return err
	}
	oldKey := hex.EncodeToString(oldTokenHash)
	newKey := hex.EncodeToString(replacement.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	oldID, exists := r.byToken[oldKey]
	if !exists {
		return domain.NewError(domain.KindNotFound, "session not found")
	}
	old := r.byID[oldID]
	if old.Revoked {
		r.revokeFamilyLocked(old.FamilyID)
		return domain.NewError(domain.KindUnauthorized, "session is revoked")
	}
	if _, exists := r.byID[replacement.ID]; exists {
		return domain.NewError(domain.KindConflict, "session ID already exists")
	}
	if _, exists := r.byToken[newKey]; exists {
		return domain.NewError(domain.KindConflict, "session token already exists")
	}
	old.Revoked = true
	r.byID[oldID] = cloneSession(old)
	r.byID[replacement.ID] = cloneSession(replacement)
	r.byToken[newKey] = replacement.ID
	return nil
}

// RevokeFamily marks every session in a token family as revoked.
func (r *SessionRepository) RevokeFamily(familyID domain.ID) error {
	if familyID == "" {
		return domain.NewError(domain.KindInvalid, "session family is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revokeFamilyLocked(familyID)
	return nil
}

func (r *SessionRepository) ListByUser(userID domain.ID) []domain.Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Session, 0)
	for _, session := range r.byID {
		if session.UserID == userID {
			result = append(result, cloneSession(session))
		}
	}
	return result
}

func (r *SessionRepository) Revoke(id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, exists := r.byID[id]
	if !exists {
		return domain.NewError(domain.KindNotFound, "session not found")
	}
	session.Revoked = true
	r.byID[id] = cloneSession(session)
	return nil
}

func (r *SessionRepository) RevokeAllByUser(userID domain.ID) error {
	if userID == "" {
		return domain.NewError(domain.KindInvalid, "user ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, session := range r.byID {
		if session.UserID == userID {
			session.Revoked = true
			r.byID[id] = cloneSession(session)
		}
	}
	return nil
}

func (r *SessionRepository) revokeFamilyLocked(familyID domain.ID) {
	for id, session := range r.byID {
		if session.FamilyID == familyID {
			session.Revoked = true
			r.byID[id] = cloneSession(session)
		}
	}
}

func validateSession(session domain.Session) error {
	if session.ID == "" || session.UserID == "" || session.FamilyID == "" {
		return domain.NewError(domain.KindInvalid, "session identifiers are required")
	}
	if len(session.TokenHash) != 32 {
		return domain.NewError(domain.KindInvalid, "session token hash is invalid")
	}
	if session.ExpiresAt.IsZero() {
		return domain.NewError(domain.KindInvalid, "session expiry is required")
	}
	return nil
}

func cloneSession(session domain.Session) domain.Session {
	session.TokenHash = append([]byte(nil), session.TokenHash...)
	return session
}
