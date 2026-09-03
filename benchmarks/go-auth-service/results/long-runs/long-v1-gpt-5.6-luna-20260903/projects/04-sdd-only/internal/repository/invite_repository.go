package repository

import (
	"encoding/hex"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type InviteStore interface {
	Create(invite domain.Invite) error
	FindByTokenHash(hash []byte) (domain.Invite, error)
	Consume(hash []byte, now time.Time) (domain.Invite, error)
}

type InviteRepository struct {
	mu      sync.Mutex
	byID    map[domain.ID]domain.Invite
	byToken map[string]domain.ID
}

func NewInviteRepository() *InviteRepository {
	return &InviteRepository{byID: make(map[domain.ID]domain.Invite), byToken: make(map[string]domain.ID)}
}

func (r *InviteRepository) Create(invite domain.Invite) error {
	if invite.ID == "" || invite.OrganizationID == "" || invite.InvitedEmail == "" || len(invite.TokenHash) != 32 || invite.ExpiresAt.IsZero() {
		return domain.NewError(domain.KindInvalid, "invite is invalid")
	}
	key := hex.EncodeToString(invite.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[invite.ID]; exists {
		return domain.NewError(domain.KindConflict, "invite ID already exists")
	}
	if _, exists := r.byToken[key]; exists {
		return domain.NewError(domain.KindConflict, "invite token already exists")
	}
	r.byID[invite.ID] = cloneInvite(invite)
	r.byToken[key] = invite.ID
	return nil
}

func (r *InviteRepository) Consume(hash []byte, now time.Time) (domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, exists := r.byToken[hex.EncodeToString(hash)]
	if !exists {
		return domain.Invite{}, domain.NewError(domain.KindUnauthorized, "invalid invite token")
	}
	invite := r.byID[id]
	if invite.Used || !now.Before(invite.ExpiresAt) {
		return domain.Invite{}, domain.NewError(domain.KindUnauthorized, "invalid invite token")
	}
	invite.Used = true
	r.byID[id] = cloneInvite(invite)
	return cloneInvite(invite), nil
}

func (r *InviteRepository) FindByTokenHash(hash []byte) (domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, exists := r.byToken[hex.EncodeToString(hash)]
	if !exists {
		return domain.Invite{}, domain.NewError(domain.KindUnauthorized, "invalid invite token")
	}
	return cloneInvite(r.byID[id]), nil
}

func cloneInvite(invite domain.Invite) domain.Invite {
	invite.TokenHash = append([]byte(nil), invite.TokenHash...)
	return invite
}
