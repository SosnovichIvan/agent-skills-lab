package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
)

const passwordIterations = 210000
const passwordSaltSize = 16

func hashPassword(password string) (salt, hash []byte, err error) {
	salt = make([]byte, passwordSaltSize)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, derivePassword([]byte(password), salt), nil
}

func verifyPassword(password string, salt, expected []byte) bool {
	actual := derivePassword([]byte(password), salt)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func derivePassword(password, salt []byte) []byte {
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], 1)
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write(counter[:])
	previous := mac.Sum(nil)
	result := append([]byte(nil), previous...)
	for i := 1; i < passwordIterations; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(previous)
		previous = mac.Sum(nil)
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result
}
