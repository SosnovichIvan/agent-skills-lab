package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

const PasswordResetTokenBytes = 32

func NewPasswordResetToken() (string, error) {
	var raw [PasswordResetTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("generate password reset token")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func HashPasswordResetToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

func ValidPasswordResetToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == PasswordResetTokenBytes
}
