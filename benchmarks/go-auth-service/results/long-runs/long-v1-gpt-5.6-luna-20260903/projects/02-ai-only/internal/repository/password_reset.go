package repository

import (
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type PasswordResetRepository struct {
	mu     sync.Mutex
	byHash map[string]domain.PasswordReset
}

func NewPasswordResetRepository() *PasswordResetRepository {
	return &PasswordResetRepository{byHash: make(map[string]domain.PasswordReset)}
}

func (r *PasswordResetRepository) Create(reset domain.PasswordReset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reset.ID == "" || reset.UserID == "" || len(reset.TokenHash) == 0 {
		return domain.Invalid("password reset is incomplete")
	}
	key := hex.EncodeToString(reset.TokenHash)
	if _, exists := r.byHash[key]; exists {
		return domain.Conflict("password reset token already exists")
	}
	reset.TokenHash = append([]byte(nil), reset.TokenHash...)
	r.byHash[key] = reset
	return nil
}

// Consume atomically checks and marks a reset token used, making it
// single-use even when two confirmations arrive concurrently.
func (r *PasswordResetRepository) Consume(hash []byte) (domain.PasswordReset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := hex.EncodeToString(hash)
	reset, exists := r.byHash[key]
	if !exists || reset.Used {
		return domain.PasswordReset{}, domain.Unauthorized("password reset token is invalid")
	}
	reset.Used = true
	r.byHash[key] = reset
	reset.TokenHash = append([]byte(nil), reset.TokenHash...)
	return reset, nil
}
