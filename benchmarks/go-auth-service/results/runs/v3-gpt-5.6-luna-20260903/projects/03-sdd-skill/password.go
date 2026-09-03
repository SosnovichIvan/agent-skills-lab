package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
)

const (
	passwordSaltSize = 16
	passwordKeySize  = 32
	passwordRounds   = 120000
)

func hashPassword(password string) (salt, hash []byte, err error) {
	salt = make([]byte, passwordSaltSize)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, derivePassword(password, salt), nil
}

func derivePassword(password string, salt []byte) []byte {
	var input [4]byte
	binary.BigEndian.PutUint32(input[:], 1)
	h := hmac.New(sha256.New, []byte(password))
	h.Write(salt)
	h.Write(input[:])
	u := h.Sum(nil)
	result := append([]byte(nil), u...)
	for i := 1; i < passwordRounds; i++ {
		h = hmac.New(sha256.New, []byte(password))
		h.Write(u)
		u = h.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}

func verifyPassword(password string, salt, expected []byte) bool {
	actual := derivePassword(password, salt)
	return len(actual) == len(expected) && subtle.ConstantTimeCompare(actual, expected) == 1
}
