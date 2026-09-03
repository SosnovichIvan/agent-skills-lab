package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// RefreshTokenBytes is the amount of entropy in a refresh token. The token is
// encoded as an opaque URL-safe value for transport to clients.
const RefreshTokenBytes = 32

// NewRefreshToken creates an opaque refresh token. Callers must not persist
// the returned value; it is intended to be given to the client once and then
// replaced by its digest in a repository.
func NewRefreshToken() (string, error) {
	var raw [RefreshTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("generate refresh token")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// GenerateRefreshToken is an explicit alias for NewRefreshToken.
func GenerateRefreshToken() (string, error) { return NewRefreshToken() }

// HashRefreshToken returns the only representation of a refresh token that
// should be retained by application storage.
func HashRefreshToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}

// ValidRefreshToken performs the inexpensive shape check used by repositories
// before hashing a presented credential. It deliberately does not reveal
// whether a token exists.
func ValidRefreshToken(token string) bool {
	if strings.TrimSpace(token) != token || token == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == RefreshTokenBytes
}
