package domain

import "time"

type EmailVerification struct {
	ID        ID        `json:"id"`
	UserID    UserID    `json:"user_id"`
	TokenHash []byte    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}
