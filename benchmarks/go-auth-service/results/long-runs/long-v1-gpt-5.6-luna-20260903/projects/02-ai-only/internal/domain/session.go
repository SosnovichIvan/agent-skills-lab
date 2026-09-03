package domain

import "time"

type Session struct {
	ID               SessionID `json:"id"`
	UserID           UserID    `json:"user_id"`
	RefreshTokenHash []byte    `json:"-"`
	ExpiresAt        time.Time `json:"expires_at"`
	FamilyID         ID        `json:"family_id"`
	Revoked          bool      `json:"revoked"`
	CreatedAt        time.Time `json:"created_at"`
}
