package domain

import "time"

// PasswordResetToken stores only the digest of a reset token and its
// single-use state.
type PasswordResetToken struct {
	ID        ID        `json:"id"`
	UserID    ID        `json:"user_id"`
	TokenHash []byte    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}
