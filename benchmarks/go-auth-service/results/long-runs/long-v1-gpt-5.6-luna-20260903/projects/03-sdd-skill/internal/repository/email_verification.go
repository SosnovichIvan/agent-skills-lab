package repository

import (
	"crypto/sha256"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

// EmailVerificationRepository stores only token digests and consumes them
// atomically. A consumed token cannot be used to verify another account.
type EmailVerificationRepository struct {
	mu     sync.Mutex
	tokens map[[sha256.Size]byte]domain.EmailVerificationToken
}

func NewEmailVerificationRepository() *EmailVerificationRepository {
	return &EmailVerificationRepository{tokens: make(map[[sha256.Size]byte]domain.EmailVerificationToken)}
}

func (r *EmailVerificationRepository) Create(token domain.EmailVerificationToken) error {
	if r == nil || token.UserID == "" || token.ExpiresAt.IsZero() || len(token.TokenHash) != sha256.Size {
		return domain.NewError(domain.ErrorInvalid, "invalid email verification token")
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

// Consume validates and atomically consumes a non-expired token.
func (r *EmailVerificationRepository) Consume(raw string, now time.Time) (domain.EmailVerificationToken, error) {
	if r == nil || raw == "" {
		return domain.EmailVerificationToken{}, domain.ErrUnauthorized
	}
	hash := domain.HashEmailVerificationToken(raw)
	var key [sha256.Size]byte
	copy(key[:], hash)
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[key]
	if !ok || token.Used || !now.Before(token.ExpiresAt) {
		return domain.EmailVerificationToken{}, domain.ErrUnauthorized
	}
	token.Used = true
	r.tokens[key] = token
	return token, nil
}

// Get returns token state for the already-verified idempotency path. It is
// internal state and never exposes the raw bearer value.
func (r *EmailVerificationRepository) Get(raw string) (domain.EmailVerificationToken, error) {
	if r == nil || raw == "" {
		return domain.EmailVerificationToken{}, domain.ErrUnauthorized
	}
	hash := domain.HashEmailVerificationToken(raw)
	var key [sha256.Size]byte
	copy(key[:], hash)
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[key]
	if !ok {
		return domain.EmailVerificationToken{}, domain.ErrUnauthorized
	}
	token.TokenHash = append([]byte(nil), token.TokenHash...)
	return token, nil
}
