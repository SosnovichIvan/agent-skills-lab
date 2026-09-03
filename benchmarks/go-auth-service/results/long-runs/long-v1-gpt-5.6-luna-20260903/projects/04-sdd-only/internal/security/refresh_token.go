package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const RefreshTokenSize = 32

// NewRefreshToken creates an opaque, URL-safe refresh token. The raw value is
// intended to be returned only to the caller that issued the session.
func NewRefreshToken() (string, error) {
	return newOpaqueToken()
}

// NewPasswordResetToken creates an opaque token for a password reset flow.
func NewPasswordResetToken() (string, error) {
	return newOpaqueToken()
}

// NewEmailVerificationToken creates an opaque token for email verification.
func NewEmailVerificationToken() (string, error) {
	return newOpaqueToken()
}

// NewInviteToken creates an opaque token for an organization invitation.
func NewInviteToken() (string, error) {
	return newOpaqueToken()
}

// NewAPIKey creates an opaque credential for an organization API key.
func NewAPIKey() (string, error) {
	return newOpaqueToken()
}

func newOpaqueToken() (string, error) {
	raw := make([]byte, RefreshTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashOpaqueToken returns the only representation that a repository should
// retain for an opaque token.
func HashOpaqueToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}
