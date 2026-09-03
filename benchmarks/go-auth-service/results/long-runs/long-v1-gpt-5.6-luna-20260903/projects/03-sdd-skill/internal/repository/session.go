package repository

import (
	"crypto/sha256"
	"sync"

	"benchmark.local/iam/internal/domain"
)

// SessionRepository is a concurrency-safe in-memory store for refresh
// sessions. Its indexes contain token digests and never the opaque token.
type SessionRepository struct {
	mu          sync.RWMutex
	sessions    map[domain.SessionID]domain.Session
	tokenIndex  map[[sha256.Size]byte]domain.SessionID
	familyIndex map[string]map[domain.SessionID]struct{}
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		sessions:    make(map[domain.SessionID]domain.Session),
		tokenIndex:  make(map[[sha256.Size]byte]domain.SessionID),
		familyIndex: make(map[string]map[domain.SessionID]struct{}),
	}
}

// Create stores a session and indexes its token digest. A session must have a
// user, ID, family, expiry, and a complete SHA-256 token digest.
func (r *SessionRepository) Create(session domain.Session) error {
	if r == nil || session.ID == "" || session.UserID == "" || session.Family == "" ||
		session.ExpiresAt.IsZero() || len(session.RefreshTokenHash) != sha256.Size {
		return domain.NewError(domain.ErrorInvalid, "invalid session")
	}
	var tokenHash [sha256.Size]byte
	copy(tokenHash[:], session.RefreshTokenHash)

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[session.ID]; exists {
		return domain.NewError(domain.ErrorConflict, "session already exists")
	}
	if _, exists := r.tokenIndex[tokenHash]; exists {
		return domain.NewError(domain.ErrorConflict, "refresh token already exists")
	}
	session.RefreshTokenHash = append([]byte(nil), tokenHash[:]...)
	r.sessions[session.ID] = session.Clone()
	r.tokenIndex[tokenHash] = session.ID
	if r.familyIndex[session.Family] == nil {
		r.familyIndex[session.Family] = make(map[domain.SessionID]struct{})
	}
	r.familyIndex[session.Family][session.ID] = struct{}{}
	return nil
}

// Rotate atomically marks the session represented by refreshToken as used and
// stores its replacement. Keeping both changes under one lock prevents two
// concurrent refresh requests from successfully rotating the same token.
func (r *SessionRepository) Rotate(refreshToken string, replacement domain.Session) error {
	if r == nil || refreshToken == "" || replacement.ID == "" || replacement.UserID == "" ||
		replacement.Family == "" || replacement.ExpiresAt.IsZero() ||
		len(replacement.RefreshTokenHash) != sha256.Size {
		return domain.NewError(domain.ErrorInvalid, "invalid session rotation")
	}
	digest := domain.RefreshTokenHash(refreshToken)
	var replacementHash [sha256.Size]byte
	copy(replacementHash[:], replacement.RefreshTokenHash)

	r.mu.Lock()
	defer r.mu.Unlock()
	oldID, ok := r.tokenIndex[digest]
	if !ok {
		return domain.ErrNotFound
	}
	old := r.sessions[oldID]
	if old.Revoked {
		// A revoked token is normally a rotated predecessor. Treating its
		// reuse as a family compromise invalidates the still-current token as
		// well. This is kept under the same lock as the lookup and rotation so
		// concurrent refresh requests cannot bypass family revocation.
		r.revokeFamilyLocked(old.Family)
		return domain.ErrUnauthorized
	}
	if old.UserID != replacement.UserID || old.Family != replacement.Family {
		return domain.NewError(domain.ErrorInvalid, "invalid session rotation")
	}
	if _, exists := r.sessions[replacement.ID]; exists {
		return domain.NewError(domain.ErrorConflict, "session already exists")
	}
	if _, exists := r.tokenIndex[replacementHash]; exists {
		return domain.NewError(domain.ErrorConflict, "refresh token already exists")
	}
	old.Revoked = true
	r.sessions[oldID] = old
	replacement.RefreshTokenHash = append([]byte(nil), replacementHash[:]...)
	r.sessions[replacement.ID] = replacement.Clone()
	r.tokenIndex[replacementHash] = replacement.ID
	if r.familyIndex[replacement.Family] == nil {
		r.familyIndex[replacement.Family] = make(map[domain.SessionID]struct{})
	}
	r.familyIndex[replacement.Family][replacement.ID] = struct{}{}
	return nil
}

func (r *SessionRepository) GetByID(id domain.SessionID) (domain.Session, error) {
	if r == nil {
		return domain.Session{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}
	return session.Clone(), nil
}

// GetByRefreshToken looks up a session using only the token's digest.
func (r *SessionRepository) GetByRefreshToken(token string) (domain.Session, error) {
	if r == nil || token == "" {
		return domain.Session{}, domain.ErrNotFound
	}
	digest := domain.RefreshTokenHash(token)
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.tokenIndex[digest]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}
	return r.sessions[id].Clone(), nil
}

func (r *SessionRepository) Revoke(id domain.SessionID) error {
	if r == nil {
		return domain.ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok {
		return domain.ErrNotFound
	}
	session.Revoked = true
	r.sessions[id] = session
	return nil
}

// RevokeFamily revokes every session belonging to a refresh-token family.
func (r *SessionRepository) RevokeFamily(family string) error {
	if r == nil || family == "" {
		return domain.NewError(domain.ErrorInvalid, "invalid session family")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.familyIndex[family]
	if !ok {
		return domain.ErrNotFound
	}
	r.revokeFamilyLocked(family)
	return nil
}

func (r *SessionRepository) revokeFamilyLocked(family string) {
	ids := r.familyIndex[family]
	for id := range ids {
		session := r.sessions[id]
		session.Revoked = true
		r.sessions[id] = session
	}
}

func (r *SessionRepository) RevokeAllForUser(userID domain.UserID) int {
	if r == nil || userID == "" {
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

func (r *SessionRepository) ListByUser(userID domain.UserID) []domain.Session {
	if r == nil || userID == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Session, 0)
	for _, session := range r.sessions {
		if session.UserID == userID {
			result = append(result, session.Clone())
		}
	}
	return result
}
