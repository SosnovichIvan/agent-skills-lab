package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const passwordIterations = 120000

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := deriveKey([]byte(password), salt, passwordIterations)
	return fmt.Sprintf("v1$%d$%s$%s", passwordIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "v1" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil || iterations < 10000 || iterations > 1000000 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err1 != nil || err2 != nil || len(salt) < 16 || len(expected) != sha256.Size {
		return false
	}
	actual := deriveKey([]byte(password), salt, iterations)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func deriveKey(password, salt []byte, iterations int) []byte {
	// PBKDF2-style iterative HMAC-SHA256 derivation, with one 32-byte block.
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

func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 128 {
		return errors.New("password must be between 8 and 128 characters")
	}
	return nil
}
