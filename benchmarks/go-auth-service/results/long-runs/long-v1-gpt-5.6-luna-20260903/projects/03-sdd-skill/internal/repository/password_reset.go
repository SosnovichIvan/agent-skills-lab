package repository

import (
	"crypto/sha256"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

// PasswordResetRepository stores only token digests. Consume atomically marks
// a token used, making a reset token a one-time credential under concurrency.
type PasswordResetRepository struct {
	mu     sync.Mutex
	tokens map[[sha256.Size]byte]domain.PasswordResetToken
}

func NewPasswordResetRepository() *PasswordResetRepository {
	return &PasswordResetRepository{tokens: make(map[[sha256.Size]byte]domain.PasswordResetToken)}
}

func (r *PasswordResetRepository) Create(token domain.PasswordResetToken) error {
	if r == nil || token.UserID == "" || token.ExpiresAt.IsZero() || len(token.TokenHash) != sha256.Size {
		return domain.NewError(domain.ErrorInvalid, "invalid password reset token")
	}
	var hash [sha256.Size]byte
	copy(hash[:], token.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tokens[hash]; exists {
		return domain.ErrConflict
	}
	token.TokenHash = append([]byte(nil), hash[:]...)
	r.tokens[hash] = token
	return nil
}

// Consume validates and atomically consumes token. Expired or already-used
// tokens are indistinguishable from unknown tokens.
func (r *PasswordResetRepository) Consume(raw string, now time.Time) (domain.PasswordResetToken, error) {
	if r == nil || raw == "" {
		return domain.PasswordResetToken{}, domain.ErrUnauthorized
	}
	hash := domain.HashPasswordResetToken(raw)
	var key [sha256.Size]byte
	copy(key[:], hash)
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[key]
	if !ok || token.Used || !now.Before(token.ExpiresAt) {
		return domain.PasswordResetToken{}, domain.ErrUnauthorized
	}
	token.Used = true
	r.tokens[key] = token
	return token, nil
}
