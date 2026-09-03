package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const RefreshTokenSize = 32

// NewRefreshToken creates an opaque value. Callers should only expose the
// returned value once; persistence must use HashRefreshToken instead.
func NewRefreshToken() (string, error) {
	raw := make([]byte, RefreshTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashRefreshToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return append([]byte(nil), hash[:]...)
}

func encodeOpaque(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}
