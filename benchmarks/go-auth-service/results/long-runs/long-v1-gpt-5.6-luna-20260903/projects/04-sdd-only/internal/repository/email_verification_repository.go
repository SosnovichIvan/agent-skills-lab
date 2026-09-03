package repository

import (
	"encoding/hex"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type EmailVerificationStore interface {
	Create(token domain.EmailVerificationToken) error
	Consume(hash []byte, now time.Time) (domain.EmailVerificationToken, error)
}

type EmailVerificationRepository struct {
	mu      sync.Mutex
	byID    map[domain.ID]domain.EmailVerificationToken
	byToken map[string]domain.ID
}

func NewEmailVerificationRepository() *EmailVerificationRepository {
	return &EmailVerificationRepository{
		byID:    make(map[domain.ID]domain.EmailVerificationToken),
		byToken: make(map[string]domain.ID),
	}
}

func (r *EmailVerificationRepository) Create(token domain.EmailVerificationToken) error {
	if token.ID == "" || token.UserID == "" || len(token.TokenHash) != 32 || token.ExpiresAt.IsZero() {
		return domain.NewError(domain.KindInvalid, "email verification token is invalid")
	}
	key := hex.EncodeToString(token.TokenHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[token.ID]; exists {
		return domain.NewError(domain.KindConflict, "email verification token ID already exists")
	}
	if _, exists := r.byToken[key]; exists {
		return domain.NewError(domain.KindConflict, "email verification token already exists")
	}
	r.byID[token.ID] = cloneEmailVerificationToken(token)
	r.byToken[key] = token.ID
	return nil
}

// Consume atomically checks a token. A previously used token is returned so a
// verified account can make a repeated confirmation idempotent.
func (r *EmailVerificationRepository) Consume(hash []byte, now time.Time) (domain.EmailVerificationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, exists := r.byToken[hex.EncodeToString(hash)]
	if !exists {
		return domain.EmailVerificationToken{}, domain.NewError(domain.KindUnauthorized, "invalid email verification token")
	}
	token := r.byID[id]
	if token.Used {
		return cloneEmailVerificationToken(token), nil
	}
	if !now.Before(token.ExpiresAt) {
		return domain.EmailVerificationToken{}, domain.NewError(domain.KindUnauthorized, "invalid email verification token")
	}
	token.Used = true
	r.byID[id] = cloneEmailVerificationToken(token)
	return cloneEmailVerificationToken(token), nil
}

func cloneEmailVerificationToken(token domain.EmailVerificationToken) domain.EmailVerificationToken {
	token.TokenHash = append([]byte(nil), token.TokenHash...)
	return token
}
