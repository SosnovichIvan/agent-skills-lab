package repository

import (
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type InviteRepository struct {
	mu      sync.RWMutex
	invites map[domain.InviteID]domain.Invite
	byHash  map[string]domain.InviteID
}

func NewInviteRepository() *InviteRepository {
	return &InviteRepository{invites: make(map[domain.InviteID]domain.Invite), byHash: make(map[string]domain.InviteID)}
}

func (r *InviteRepository) Create(invite domain.Invite) error {
	if invite.ID == "" || invite.OrganizationID == "" || invite.InvitedBy == "" || len(invite.TokenHash) == 0 {
		return domain.NewInvalid("invalid invite")
	}
	key := hex.EncodeToString(invite.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.invites[invite.ID]; ok || r.byHash[key] != "" {
		return domain.NewConflict("invite already exists")
	}
	r.invites[invite.ID] = invite
	r.byHash[key] = invite.ID
	return nil
}

func (r *InviteRepository) GetByToken(tokenHash []byte) (domain.Invite, error) {
	key := hex.EncodeToString(tokenHash)
	r.mu.RLock()
	id, ok := r.byHash[key]
	invite := r.invites[id]
	r.mu.RUnlock()
	if !ok || subtle.ConstantTimeCompare(invite.TokenHash, tokenHash) != 1 || !invite.AcceptedAt.IsZero() {
		return domain.Invite{}, domain.NewNotFound("invite not found")
	}
	return invite, nil
}

// Consume atomically marks an invite accepted, preventing replay and races.
func (r *InviteRepository) Consume(tokenHash []byte, acceptedAt time.Time) (domain.Invite, error) {
	key := hex.EncodeToString(tokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[key]
	if !ok {
		return domain.Invite{}, domain.NewNotFound("invite not found")
	}
	invite := r.invites[id]
	if subtle.ConstantTimeCompare(invite.TokenHash, tokenHash) != 1 || !invite.AcceptedAt.IsZero() {
		return domain.Invite{}, domain.NewConflict("invite already accepted")
	}
	invite.AcceptedAt = acceptedAt
	r.invites[id] = invite
	return invite, nil
}
