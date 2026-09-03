package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

const (
	saltSize   = 16
	iterations = 120000
)

var ErrInvalidMaterial = errors.New("invalid password material")

func Hash(value string) (salt, digest []byte, err error) {
	salt = make([]byte, saltSize)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, err
	}
	digest = derive([]byte(value), salt)
	return salt, digest, nil
}

func Verify(value string, salt, expected []byte) bool {
	if len(salt) != saltSize || len(expected) != sha256.Size {
		return false
	}
	actual := derive([]byte(value), salt)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func derive(value, salt []byte) []byte {
	var previous [sha256.Size]byte
	mac := hmac.New(sha256.New, value)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	copy(previous[:], mac.Sum(nil))
	result := previous
	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, value)
		mac.Write(previous[:])
		copy(previous[:], mac.Sum(nil))
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result[:]
}
