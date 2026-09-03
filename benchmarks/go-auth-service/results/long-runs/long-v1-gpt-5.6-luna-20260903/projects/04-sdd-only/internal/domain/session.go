package domain

import "time"

// Session is the stored metadata for a refresh-token session. TokenHash is
// the SHA-256 digest of the opaque token; the raw token is never part of this
// model.
type Session struct {
	ID        ID        `json:"id"`
	UserID    ID        `json:"user_id"`
	TokenHash []byte    `json:"-"`
	FamilyID  ID        `json:"family_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

// SessionProfile is the safe representation returned by session management
// endpoints. It intentionally excludes the token hash and user identifier.
type SessionProfile struct {
	ID        ID        `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

func (s Session) PublicProfile() SessionProfile {
	return SessionProfile{ID: s.ID, ExpiresAt: s.ExpiresAt, Revoked: s.Revoked}
}
