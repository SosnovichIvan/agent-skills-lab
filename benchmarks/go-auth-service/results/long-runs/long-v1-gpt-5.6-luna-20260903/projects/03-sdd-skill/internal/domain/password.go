package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
)

const (
	// PasswordSaltSize is large enough to make salt collisions negligible.
	PasswordSaltSize = 16

	// DefaultPasswordIterations deliberately makes password guessing expensive
	// while keeping the value encoded with each password record for upgrades.
	DefaultPasswordIterations uint32 = 120000
)

// HashPassword creates password material suitable for storage. The password
// itself is never returned or kept in the resulting value.
func HashPassword(password string) (PasswordMaterial, error) {
	if err := ValidatePassword(password); err != nil {
		return PasswordMaterial{}, err
	}

	salt := make([]byte, PasswordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return PasswordMaterial{}, NewError(ErrorInvalid, "could not create password material")
	}
	hash := derivePassword(password, salt, DefaultPasswordIterations)
	return PasswordMaterial{
		Salt:       salt,
		Hash:       hash,
		Iterations: DefaultPasswordIterations,
	}, nil
}

// VerifyPassword checks a password against stored password material. A
// malformed record is treated as a failed verification, and never panics.
func VerifyPassword(password string, material PasswordMaterial) bool {
	if len(material.Salt) == 0 || len(material.Hash) != sha256.Size || material.Iterations == 0 {
		return false
	}
	actual := derivePassword(password, material.Salt, material.Iterations)
	return subtle.ConstantTimeCompare(actual, material.Hash) == 1
}

// derivePassword is a PBKDF2-shaped construction using HMAC-SHA256. Each
// block is iterated and XORed, so every password record has an explicit work
// factor and can be rehashed when the configured cost changes.
func derivePassword(password string, salt []byte, iterations uint32) []byte {
	passwordBytes := []byte(password)
	defer clearBytes(passwordBytes)

	blockInput := make([]byte, len(salt)+4)
	copy(blockInput, salt)
	binary.BigEndian.PutUint32(blockInput[len(salt):], 1)

	mac := hmac.New(sha256.New, passwordBytes)
	_, _ = mac.Write(blockInput)
	previous := mac.Sum(nil)
	derived := append([]byte(nil), previous...)

	for i := uint32(1); i < iterations; i++ {
		mac.Reset()
		_, _ = mac.Write(previous)
		current := mac.Sum(nil)
		for j := range derived {
			derived[j] ^= current[j]
		}
		clearBytes(previous)
		previous = current
	}
	clearBytes(previous)
	return derived
}

func clearBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
