package repository

import (
	"crypto/sha256"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

// InviteRepository stores only SHA-256 digests and consumes invites atomically.
type InviteRepository struct {
	mu      sync.RWMutex
	invites map[[sha256.Size]byte]domain.InviteToken
}

func NewInviteRepository() *InviteRepository {
	return &InviteRepository{invites: make(map[[sha256.Size]byte]domain.InviteToken)}
}

func (r *InviteRepository) Create(invite domain.InviteToken) error {
	if r == nil || invite.OrganizationID == "" || invite.Email == "" || invite.ExpiresAt.IsZero() || len(invite.TokenHash) != sha256.Size {
		return domain.NewError(domain.ErrorInvalid, "invalid invite token")
	}
	var key [sha256.Size]byte
	copy(key[:], invite.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.invites[key]; exists {
		return domain.ErrConflict
	}
	invite.TokenHash = append([]byte(nil), key[:]...)
	r.invites[key] = invite
	return nil
}

// Get returns invite metadata without exposing a raw token.
func (r *InviteRepository) Get(raw string) (domain.InviteToken, error) {
	if r == nil || raw == "" {
		return domain.InviteToken{}, domain.ErrUnauthorized
	}
	var key [sha256.Size]byte
	copy(key[:], domain.HashInviteToken(raw))
	r.mu.RLock()
	defer r.mu.RUnlock()
	invite, ok := r.invites[key]
	if !ok {
		return domain.InviteToken{}, domain.ErrUnauthorized
	}
	return invite.Clone(), nil
}

func (r *InviteRepository) Consume(raw string, now time.Time) (domain.InviteToken, error) {
	if r == nil || raw == "" {
		return domain.InviteToken{}, domain.ErrUnauthorized
	}
	var key [sha256.Size]byte
	copy(key[:], domain.HashInviteToken(raw))
	r.mu.Lock()
	defer r.mu.Unlock()
	invite, ok := r.invites[key]
	if !ok || invite.Used || !now.Before(invite.ExpiresAt) {
		return domain.InviteToken{}, domain.ErrUnauthorized
	}
	invite.Used = true
	r.invites[key] = invite
	return invite.Clone(), nil
}
