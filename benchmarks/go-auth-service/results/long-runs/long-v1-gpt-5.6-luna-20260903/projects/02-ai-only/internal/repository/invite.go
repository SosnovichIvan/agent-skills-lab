package repository

import (
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type InviteRepository struct {
	mu     sync.Mutex
	byHash map[string]domain.Invite
}

func NewInviteRepository() *InviteRepository {
	return &InviteRepository{byHash: make(map[string]domain.Invite)}
}

func (r *InviteRepository) Create(invite domain.Invite) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if invite.ID == "" || invite.OrganizationID == "" || invite.InvitedEmail == "" || invite.Role == "" || len(invite.TokenHash) == 0 {
		return domain.Invalid("invite is incomplete")
	}
	key := hex.EncodeToString(invite.TokenHash)
	if _, exists := r.byHash[key]; exists {
		return domain.Conflict("invite token already exists")
	}
	invite.TokenHash = append([]byte(nil), invite.TokenHash...)
	r.byHash[key] = invite
	return nil
}

func (r *InviteRepository) Consume(hash []byte) (domain.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := hex.EncodeToString(hash)
	invite, exists := r.byHash[key]
	if !exists {
		return domain.Invite{}, domain.Unauthorized("invite token is invalid")
	}
	if invite.Used {
		return cloneInvite(invite), domain.ErrTokenReuse
	}
	invite.Used = true
	r.byHash[key] = invite
	return cloneInvite(invite), nil
}

func cloneInvite(invite domain.Invite) domain.Invite {
	invite.TokenHash = append([]byte(nil), invite.TokenHash...)
	return invite
}
