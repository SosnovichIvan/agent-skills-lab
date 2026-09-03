package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const EmailVerificationTokenSize = 32

// EmailVerificationToken is server-side state. Only the digest of the bearer
// token is retained.
type EmailVerificationToken struct {
	TokenHash []byte
	UserID    UserID
	ExpiresAt time.Time
	Used      bool
}

func NewEmailVerificationToken() (string, error) {
	value := make([]byte, EmailVerificationTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", NewError(ErrorInvalid, "could not create email verification token")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func HashEmailVerificationToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}
