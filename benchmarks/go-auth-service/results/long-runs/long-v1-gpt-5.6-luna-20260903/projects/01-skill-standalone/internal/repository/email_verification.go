package repository

import (
	"bytes"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type EmailVerification struct {
	UserID    domain.UserID
	TokenHash []byte
	ExpiresAt time.Time
	Used      bool
}

type EmailVerificationRepository struct {
	mu     sync.Mutex
	tokens map[[32]byte]EmailVerification
}

func NewEmailVerificationRepository() *EmailVerificationRepository {
	return &EmailVerificationRepository{tokens: make(map[[32]byte]EmailVerification)}
}

func (r *EmailVerificationRepository) Create(userID domain.UserID, token string, expiresAt time.Time) error {
	if userID == "" || !domain.ValidEmailVerificationToken(token) || expiresAt.IsZero() {
		return domain.NewInvalid("invalid email verification token")
	}
	digest := domain.HashEmailVerificationToken(token)
	var key [32]byte
	copy(key[:], digest)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tokens[key]; exists {
		return domain.NewConflict("email verification token already exists")
	}
	r.tokens[key] = EmailVerification{UserID: userID, TokenHash: bytes.Clone(digest), ExpiresAt: expiresAt}
	return nil
}

func (r *EmailVerificationRepository) Consume(token string, now time.Time) (domain.UserID, error) {
	if !domain.ValidEmailVerificationToken(token) {
		return "", domain.NewUnauthorized("invalid email verification token")
	}
	digest := domain.HashEmailVerificationToken(token)
	var key [32]byte
	copy(key[:], digest)
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.tokens[key]
	if !ok || record.Used || !now.Before(record.ExpiresAt) {
		return "", domain.NewUnauthorized("invalid email verification token")
	}
	record.Used = true
	r.tokens[key] = record
	return record.UserID, nil
}

// Get is used only to preserve idempotent confirmation after a consumed token.
// It returns a detached record so callers cannot mutate repository state.
func (r *EmailVerificationRepository) Get(token string) (EmailVerification, error) {
	if !domain.ValidEmailVerificationToken(token) {
		return EmailVerification{}, domain.NewUnauthorized("invalid email verification token")
	}
	digest := domain.HashEmailVerificationToken(token)
	var key [32]byte
	copy(key[:], digest)
	r.mu.Lock()
	record, ok := r.tokens[key]
	r.mu.Unlock()
	if !ok {
		return EmailVerification{}, domain.NewUnauthorized("invalid email verification token")
	}
	record.TokenHash = bytes.Clone(record.TokenHash)
	return record, nil
}
