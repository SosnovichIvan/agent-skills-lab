package domain

import "time"

// EmailVerificationToken stores only a digest and single-use metadata.
type EmailVerificationToken struct {
	ID        ID        `json:"id"`
	UserID    ID        `json:"user_id"`
	TokenHash []byte    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}
