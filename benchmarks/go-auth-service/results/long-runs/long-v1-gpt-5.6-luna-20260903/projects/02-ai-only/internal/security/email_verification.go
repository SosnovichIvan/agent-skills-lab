package security

import "crypto/rand"

const EmailVerificationTokenSize = 32

func NewEmailVerificationToken() (string, error) {
	raw := make([]byte, EmailVerificationTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return encodeOpaque(raw), nil
}

func HashEmailVerificationToken(token string) []byte {
	return HashRefreshToken(token)
}
