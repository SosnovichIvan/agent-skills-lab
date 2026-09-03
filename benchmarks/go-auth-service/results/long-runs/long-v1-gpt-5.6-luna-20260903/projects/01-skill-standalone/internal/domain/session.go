package domain

import "time"

// Session is the server-side state associated with one refresh-token family
// member. RefreshTokenHash is intentionally excluded from JSON responses.
type Session struct {
	ID               SessionID     `json:"id"`
	UserID           UserID        `json:"user_id"`
	RefreshTokenHash []byte        `json:"-"`
	ExpiresAt        time.Time     `json:"expires_at"`
	TokenFamilyID    TokenFamilyID `json:"token_family_id"`
	Revoked          bool          `json:"revoked"`
	CreatedAt        time.Time     `json:"created_at"`
}
