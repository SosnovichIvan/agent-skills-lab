package repository

import (
	"bytes"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type PasswordReset struct {
	UserID    domain.UserID
	TokenHash []byte
	ExpiresAt time.Time
	Used      bool
}

type PasswordResetRepository struct {
	mu     sync.Mutex
	tokens map[[32]byte]PasswordReset
}

func NewPasswordResetRepository() *PasswordResetRepository {
	return &PasswordResetRepository{tokens: make(map[[32]byte]PasswordReset)}
}

func (r *PasswordResetRepository) Create(userID domain.UserID, token string, expiresAt time.Time) error {
	if userID == "" || !domain.ValidPasswordResetToken(token) || expiresAt.IsZero() {
		return domain.NewInvalid("invalid password reset token")
	}
	digest := domain.HashPasswordResetToken(token)
	var key [32]byte
	copy(key[:], digest)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tokens[key]; exists {
		return domain.NewConflict("password reset token already exists")
	}
	r.tokens[key] = PasswordReset{UserID: userID, TokenHash: bytes.Clone(digest), ExpiresAt: expiresAt}
	return nil
}

func (r *PasswordResetRepository) Consume(token string, now time.Time) (domain.UserID, error) {
	if !domain.ValidPasswordResetToken(token) {
		return "", domain.NewUnauthorized("invalid password reset token")
	}
	digest := domain.HashPasswordResetToken(token)
	var key [32]byte
	copy(key[:], digest)
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.tokens[key]
	if !ok || record.Used || !now.Before(record.ExpiresAt) {
		return "", domain.NewUnauthorized("invalid password reset token")
	}
	record.Used = true
	r.tokens[key] = record
	return record.UserID, nil
}
