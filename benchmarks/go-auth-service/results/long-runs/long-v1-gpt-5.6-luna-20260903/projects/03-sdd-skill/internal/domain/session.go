package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const RefreshTokenSize = 32

// Session is the server-side state for a refresh-token session. The token
// itself is never part of the session: only its SHA-256 digest is retained.
type Session struct {
	ID               SessionID `json:"id"`
	UserID           UserID    `json:"user_id"`
	RefreshTokenHash []byte    `json:"-"`
	ExpiresAt        time.Time `json:"expires_at"`
	Family           string    `json:"family"`
	Revoked          bool      `json:"revoked"`
}

// NewRefreshToken creates an opaque, URL-safe refresh token. Its value is
// intended to be returned to the client once and then discarded by the
// server after HashRefreshToken has been called.
func NewRefreshToken() (string, error) {
	value := make([]byte, RefreshTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", NewError(ErrorInvalid, "could not create refresh token")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

// HashRefreshToken returns the digest suitable for session persistence.
// Callers should not persist or log the supplied token.
func HashRefreshToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}

// RefreshTokenHash is the fixed-size form of HashRefreshToken for callers
// that prefer a value that cannot be resized or appended to accidentally.
func RefreshTokenHash(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

// Clone returns an independent session value, including its token digest.
func (s Session) Clone() Session {
	s.RefreshTokenHash = append([]byte(nil), s.RefreshTokenHash...)
	return s
}
