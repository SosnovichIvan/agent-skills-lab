package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

const MinPasswordLength = 12

// NormalizeEmail trims surrounding whitespace, lowercases the address, and
// rejects malformed addresses. Display names are not accepted as email
// values because this is an account identifier, not a mail header.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", Invalid("email is required")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", Invalid("email is invalid")
	}
	return email, nil
}

// ValidatePassword enforces the minimum password length without putting the
// supplied secret in the returned error.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return Invalid("password is invalid")
	}
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return Invalid("password must be at least 12 characters")
	}
	return nil
}
