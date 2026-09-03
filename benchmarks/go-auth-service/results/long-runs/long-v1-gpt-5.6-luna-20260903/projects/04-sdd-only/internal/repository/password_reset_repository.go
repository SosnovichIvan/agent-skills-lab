package repository

import (
	"encoding/hex"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type PasswordResetStore interface {
	Create(token domain.PasswordResetToken) error
	Consume(hash []byte, now time.Time) (domain.PasswordResetToken, error)
}

type PasswordResetRepository struct {
	mu      sync.Mutex
	byID    map[domain.ID]domain.PasswordResetToken
	byToken map[string]domain.ID
}

func NewPasswordResetRepository() *PasswordResetRepository {
	return &PasswordResetRepository{
		byID:    make(map[domain.ID]domain.PasswordResetToken),
		byToken: make(map[string]domain.ID),
	}
}

func (r *PasswordResetRepository) Create(token domain.PasswordResetToken) error {
	if token.ID == "" || token.UserID == "" || len(token.TokenHash) != 32 || token.ExpiresAt.IsZero() {
		return domain.NewError(domain.KindInvalid, "password reset token is invalid")
	}
	key := hex.EncodeToString(token.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[token.ID]; exists {
		return domain.NewError(domain.KindConflict, "password reset token ID already exists")
	}
	if _, exists := r.byToken[key]; exists {
		return domain.NewError(domain.KindConflict, "password reset token already exists")
	}
	r.byID[token.ID] = clonePasswordResetToken(token)
	r.byToken[key] = token.ID
	return nil
}

// Consume atomically validates and marks a reset token as used.
func (r *PasswordResetRepository) Consume(hash []byte, now time.Time) (domain.PasswordResetToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, exists := r.byToken[hex.EncodeToString(hash)]
	if !exists {
		return domain.PasswordResetToken{}, domain.NewError(domain.KindUnauthorized, "invalid password reset token")
	}
	token := r.byID[id]
	if token.Used || !now.Before(token.ExpiresAt) {
		return domain.PasswordResetToken{}, domain.NewError(domain.KindUnauthorized, "invalid password reset token")
	}
	token.Used = true
	r.byID[id] = clonePasswordResetToken(token)
	return clonePasswordResetToken(token), nil
}

func clonePasswordResetToken(token domain.PasswordResetToken) domain.PasswordResetToken {
	token.TokenHash = append([]byte(nil), token.TokenHash...)
	return token
}
