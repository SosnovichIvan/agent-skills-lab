package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
)

const (
	// PasswordSaltSize is the size of the random salt stored with a password
	// hash.
	PasswordSaltSize = 32
	// PasswordHashSize is the output size of HMAC-SHA256.
	PasswordHashSize = sha256.Size
	// PasswordKDFIterations makes password guessing deliberately expensive.
	PasswordKDFIterations uint32 = 120000
)

// HashPassword derives password material using PBKDF2-style iterative
// HMAC-SHA256. Only the salt, derived hash, and work factor are retained; the
// cleartext password is never placed in the returned value.
func HashPassword(password string) (PasswordMaterial, error) {
	if err := ValidatePassword(password); err != nil {
		return PasswordMaterial{}, err
	}

	salt := make([]byte, PasswordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return PasswordMaterial{}, errors.New("generate password salt")
	}
	hash := derivePasswordKey([]byte(password), salt, PasswordKDFIterations)
	return PasswordMaterial{
		Salt:       salt,
		Hash:       hash,
		Iterations: PasswordKDFIterations,
	}, nil
}

// VerifyPassword checks a password without exposing password-derived data.
// Invalid or incomplete stored material is treated as a failed check.
func VerifyPassword(password string, material PasswordMaterial) bool {
	if ValidatePassword(password) != nil || len(material.Salt) == 0 ||
		len(material.Hash) != PasswordHashSize || material.Iterations == 0 {
		return false
	}
	derived := derivePasswordKey([]byte(password), material.Salt, material.Iterations)
	return subtle.ConstantTimeCompare(derived, material.Hash) == 1
}

func derivePasswordKey(password, salt []byte, iterations uint32) []byte {
	// U_1 = PRF(password, salt || INT_32_BE(1)); each later U is chained
	// through the previous result and all U values are XORed into T.
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], 1)
	mac.Write(counter[:])
	previous := mac.Sum(nil)
	result := append([]byte(nil), previous...)
	for i := uint32(1); i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(previous)
		previous = mac.Sum(nil)
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result
}
