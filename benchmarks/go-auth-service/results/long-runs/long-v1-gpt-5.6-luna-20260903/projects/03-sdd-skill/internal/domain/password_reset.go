package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const PasswordResetTokenSize = 32

// PasswordResetToken is server-side state. The bearer token is never a field
// of this type; persistence receives only its SHA-256 digest.
type PasswordResetToken struct {
	TokenHash []byte
	UserID    UserID
	ExpiresAt time.Time
	Used      bool
}

func NewPasswordResetToken() (string, error) {
	value := make([]byte, PasswordResetTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", NewError(ErrorInvalid, "could not create password reset token")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func HashPasswordResetToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}
