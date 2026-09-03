package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

const (
	passwordSaltSize = 16
	passwordKeySize  = 32
	passwordRounds   = 120000
)

func hashPassword(password string) ([]byte, []byte, error) {
	salt := make([]byte, passwordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, derivePassword(password, salt), nil
}

func derivePassword(password string, salt []byte) []byte {
	key := []byte(password)
	block := make([]byte, len(salt)+4)
	copy(block, salt)
	block[len(salt)+3] = 1
	var result [sha256.Size]byte
	h := hmac.New(sha256.New, key)
	h.Write(block)
	u := h.Sum(nil)
	copy(result[:], u)
	for i := 1; i < passwordRounds; i++ {
		h = hmac.New(sha256.New, key)
		h.Write(u)
		u = h.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result[:]
}

func checkPassword(password string, salt, expected []byte) error {
	if len(salt) != passwordSaltSize || len(expected) != passwordKeySize {
		return errors.New("invalid password material")
	}
	actual := derivePassword(password, salt)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return errors.New("invalid credentials")
	}
	return nil
}
