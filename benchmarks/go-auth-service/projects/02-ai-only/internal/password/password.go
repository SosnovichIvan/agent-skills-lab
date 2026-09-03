package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	algorithm  = "hmac-sha256"
	iterations = 120000
	saltSize   = 16
)

func ValidatePlaintext(value string) error {
	n := utf8.RuneCountInString(value)
	if n < 8 || n > 128 {
		return errors.New("password must contain between 8 and 128 characters")
	}
	return nil
}

func Hash(value string) (string, error) {
	if err := ValidatePlaintext(value); err != nil {
		return "", err
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	derived := derive([]byte(value), salt, iterations)
	return fmt.Sprintf("v1$%s$%d$%s$%s", algorithm, iterations, base64.RawURLEncoding.EncodeToString(salt), base64.RawURLEncoding.EncodeToString(derived)), nil
}

func Compare(value, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "v1" || parts[1] != algorithm {
		return false
	}
	count, err := strconv.Atoi(parts[2])
	if err != nil || count < 10000 || count > 1000000 {
		return false
	}
	saltText, hashText := parts[3], parts[4]
	salt, err1 := base64.RawURLEncoding.DecodeString(saltText)
	expected, err2 := base64.RawURLEncoding.DecodeString(hashText)
	if err1 != nil || err2 != nil || len(salt) < 8 || len(expected) != sha256.Size {
		return false
	}
	actual := derive([]byte(value), salt, count)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func derive(value, salt []byte, count int) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(value)
	mac.Write([]byte{0, 0, 0, 1})
	u := mac.Sum(nil)
	out := append([]byte(nil), u...)
	for i := 1; i < count; i++ {
		mac = hmac.New(sha256.New, salt)
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}
