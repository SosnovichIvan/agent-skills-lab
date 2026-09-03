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
	DefaultPasswordIterations = 120000
	DefaultPasswordSaltSize   = 32
)

// PasswordHasher derives password material with iterative HMAC-SHA256.
type PasswordHasher struct {
	Iterations int
	SaltSize   int
}

// NewPasswordHasher constructs a hasher with explicit work-factor settings.
func NewPasswordHasher(iterations, saltSize int) (PasswordHasher, error) {
	if iterations <= 0 {
		return PasswordHasher{}, errors.New("password iterations must be positive")
	}
	if saltSize <= 0 {
		return PasswordHasher{}, errors.New("password salt size must be positive")
	}
	return PasswordHasher{Iterations: iterations, SaltSize: saltSize}, nil
}

func defaultPasswordHasher() PasswordHasher {
	return PasswordHasher{Iterations: DefaultPasswordIterations, SaltSize: DefaultPasswordSaltSize}
}

// HashPassword validates and derives password material. The returned value
// contains no copy of the cleartext password.
func (h PasswordHasher) HashPassword(password string) (domain.PasswordMaterial, error) {
	if err := ValidatePassword(password); err != nil {
		return domain.PasswordMaterial{}, err
	}
	if h.Iterations <= 0 {
		h.Iterations = DefaultPasswordIterations
	}
	if h.SaltSize <= 0 {
		h.SaltSize = DefaultPasswordSaltSize
	}
	salt := make([]byte, h.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return domain.PasswordMaterial{}, errors.New("generate password salt")
	}
	return domain.PasswordMaterial{
		Salt:       salt,
		Hash:       deriveKey(password, salt, h.Iterations),
		Iterations: h.Iterations,
	}, nil
}

// VerifyPassword derives a candidate and compares it in constant time.
func (h PasswordHasher) VerifyPassword(password string, material domain.PasswordMaterial) bool {
	if !utf8PasswordCandidate(password) || len(material.Salt) == 0 || len(material.Hash) != sha256.Size || material.Iterations <= 0 {
		return false
	}
	candidate := deriveKey(password, material.Salt, material.Iterations)
	return subtle.ConstantTimeCompare(candidate, material.Hash) == 1
}

// HashPassword uses the production work factor.
func HashPassword(password string) (domain.PasswordMaterial, error) {
	return defaultPasswordHasher().HashPassword(password)
}

// VerifyPassword uses the iteration count recorded in the material.
func VerifyPassword(password string, material domain.PasswordMaterial) bool {
	return defaultPasswordHasher().VerifyPassword(password, material)
}

func deriveKey(password string, salt []byte, iterations int) []byte {
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], 1)

	h := hmac.New(sha256.New, []byte(password))
	h.Write(salt)
	h.Write(counter[:])
	previous := h.Sum(nil)
	result := append([]byte(nil), previous...)
	for i := 1; i < iterations; i++ {
		h = hmac.New(sha256.New, []byte(password))
		h.Write(previous)
		previous = h.Sum(nil)
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result
}

func utf8PasswordCandidate(password string) bool {
	return ValidatePassword(password) == nil
}
