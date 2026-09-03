package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

const EmailVerificationTokenBytes = 32

// NewEmailVerificationToken creates an opaque, URL-safe token. Only its hash
// is suitable for persistence; the clear value is returned to the caller once.
func NewEmailVerificationToken() (string, error) {
	var raw [EmailVerificationTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("generate email verification token")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func HashEmailVerificationToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

func ValidEmailVerificationToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == EmailVerificationTokenBytes
}
