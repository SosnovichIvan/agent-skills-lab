package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

const passwordIterations = 120000

type passwordHash struct {
	Salt       []byte
	DerivedKey []byte
	Iterations int
}

func hashPassword(password string) (passwordHash, error) {
	if len(password) < 8 || len(password) > 256 {
		return passwordHash{}, errors.New("password must be between 8 and 256 characters")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return passwordHash{}, fmt.Errorf("generate password salt: %w", e)
	}
	return passwordHash{salt, deriveKey([]byte(password), salt, passwordIterations), passwordIterations}, nil
}
func (p passwordHash) verify(password string) bool {
	return p.Iterations > 0 && len(p.Salt) > 0 && subtle.ConstantTimeCompare(deriveKey([]byte(password), p.Salt, p.Iterations), p.DerivedKey) == 1
}
func deriveKey(password, salt []byte, iterations int) []byte {
	b := make([]byte, len(salt)+4)
	copy(b, salt)
	b[len(salt)+3] = 1
	u := hmacSHA256(password, b)
	out := append([]byte(nil), u...)
	for i := 1; i < iterations; i++ {
		u = hmacSHA256(password, u)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}
func hmacSHA256(k, v []byte) []byte {
	h := hmac.New(sha256.New, k)
	_, _ = h.Write(v)
	return h.Sum(nil)
}
func encodeBytes(v []byte) string { return base64.RawURLEncoding.EncodeToString(v) }
