// Package security contains credential primitives used by the IAM service.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"

	"benchmark.local/iam/internal/domain"
)

const (
	DefaultPasswordIterations = 120_000
	DefaultSaltSize           = 32
)

var ErrPasswordMismatch = errors.New("password mismatch")

// HashPassword creates password material. The password itself is never
// included in the returned value.
func HashPassword(password string) (domain.PasswordMaterial, error) {
	return HashPasswordWithParams(password, DefaultPasswordIterations, DefaultSaltSize)
}

func HashPasswordWithParams(password string, iterations, saltSize int) (domain.PasswordMaterial, error) {
	if err := domain.ValidatePassword(password); err != nil {
		return domain.PasswordMaterial{}, err
	}
	if iterations <= 0 || saltSize <= 0 {
		return domain.PasswordMaterial{}, domain.Invalid("password KDF parameters are invalid")
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return domain.PasswordMaterial{}, errors.New("password salt generation failed")
	}
	return domain.PasswordMaterial{
		Salt:       salt,
		Hash:       derive(password, salt, iterations),
		Iterations: iterations,
		Algorithm:  "HMAC-SHA256",
	}, nil
}

// VerifyPassword checks a password against stored material. It returns only
// whether verification succeeded; no credential material is returned.
func VerifyPassword(material domain.PasswordMaterial, password string) bool {
	if material.Algorithm != "HMAC-SHA256" || len(material.Salt) == 0 || len(material.Hash) == 0 || material.Iterations <= 0 {
		return false
	}
	if domain.ValidatePassword(password) != nil {
		return false
	}
	actual := derive(password, material.Salt, material.Iterations)
	return subtle.ConstantTimeCompare(actual, material.Hash) == 1
}

func CheckPassword(material domain.PasswordMaterial, password string) error {
	if !VerifyPassword(material, password) {
		return ErrPasswordMismatch
	}
	return nil
}

// derive is PBKDF2-shaped iterative HMAC-SHA256 for the single block needed
// by password material. Each round depends on the previous round.
func derive(password string, salt []byte, iterations int) []byte {
	mac := hmac.New(sha256.New, []byte(password))
	_, _ = mac.Write(salt)
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], 1)
	_, _ = mac.Write(counter[:])
	previous := mac.Sum(nil)
	result := append([]byte(nil), previous...)
	for round := 1; round < iterations; round++ {
		mac = hmac.New(sha256.New, []byte(password))
		_, _ = mac.Write(previous)
		previous = mac.Sum(nil)
		for i := range result {
			result[i] ^= previous[i]
		}
	}
	return result
}
