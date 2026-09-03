package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
)

const (
	SaltSize   = 16
	KeySize    = 32
	Iterations = 120000
)

type Hasher struct{}

type Hash struct {
	Salt       []byte
	DerivedKey []byte
	Iterations int
}

func New() Hasher { return Hasher{} }

func (Hasher) Hash(plain string) (Hash, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return Hash{}, fmt.Errorf("generate password salt: %w", err)
	}
	return Hash{Salt: salt, DerivedKey: derive([]byte(plain), salt, Iterations), Iterations: Iterations}, nil
}

func (Hasher) Verify(plain string, stored Hash) bool {
	if len(stored.Salt) != SaltSize || len(stored.DerivedKey) != KeySize || stored.Iterations < 1 {
		return false
	}
	actual := derive([]byte(plain), stored.Salt, stored.Iterations)
	return subtle.ConstantTimeCompare(actual, stored.DerivedKey) == 1
}

func derive(password, salt []byte, iterations int) []byte {
	// PBKDF2-style iterative HMAC-SHA256 derivation for a single 32-byte block.
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	previous := mac.Sum(nil)
	result := append([]byte(nil), previous...)
	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(previous)
		previous = mac.Sum(nil)
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result
}

func Validate(plain string) error {
	if len([]byte(plain)) < 8 || len([]byte(plain)) > 128 {
		return errors.New("password must contain between 8 and 128 bytes")
	}
	return nil
}
