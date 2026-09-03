package security

import (
	"crypto/rand"
	"encoding/base64"
)

const PasswordResetTokenSize = 32

func NewPasswordResetToken() (string, error) {
	raw := make([]byte, PasswordResetTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashPasswordResetToken(token string) []byte {
	return HashRefreshToken(token)
}
