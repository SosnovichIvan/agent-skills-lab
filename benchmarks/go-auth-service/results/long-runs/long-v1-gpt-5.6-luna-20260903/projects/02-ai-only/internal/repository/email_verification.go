package repository

import (
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type EmailVerificationRepository struct {
	mu     sync.Mutex
	byHash map[string]domain.EmailVerification
}

func NewEmailVerificationRepository() *EmailVerificationRepository {
	return &EmailVerificationRepository{byHash: make(map[string]domain.EmailVerification)}
}

func (r *EmailVerificationRepository) Create(verification domain.EmailVerification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if verification.ID == "" || verification.UserID == "" || len(verification.TokenHash) == 0 {
		return domain.Invalid("email verification is incomplete")
	}
	key := hex.EncodeToString(verification.TokenHash)
	if _, exists := r.byHash[key]; exists {
		return domain.Conflict("email verification token already exists")
	}
	verification.TokenHash = append([]byte(nil), verification.TokenHash...)
	r.byHash[key] = verification
	return nil
}

// Consume marks a token used under the repository lock. Used records are
// returned with ErrTokenReuse so callers can implement idempotent confirmation
// for an already verified user without making the token reusable.
func (r *EmailVerificationRepository) Consume(hash []byte) (domain.EmailVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	verification, exists := r.byHash[hex.EncodeToString(hash)]
	if !exists {
		return domain.EmailVerification{}, domain.Unauthorized("email verification token is invalid")
	}
	if verification.Used {
		return cloneVerification(verification), domain.ErrTokenReuse
	}
	verification.Used = true
	r.byHash[hex.EncodeToString(hash)] = verification
	return cloneVerification(verification), nil
}

func cloneVerification(verification domain.EmailVerification) domain.EmailVerification {
	verification.TokenHash = append([]byte(nil), verification.TokenHash...)
	return verification
}
