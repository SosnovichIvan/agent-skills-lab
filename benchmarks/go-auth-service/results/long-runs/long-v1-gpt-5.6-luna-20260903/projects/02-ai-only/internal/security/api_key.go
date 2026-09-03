package security

import "crypto/rand"

const APIKeySize = 32

func NewAPIKey() (string, error) {
	raw := make([]byte, APIKeySize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return encodeOpaque(raw), nil
}

func HashAPIKey(key string) []byte { return HashRefreshToken(key) }
