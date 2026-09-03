package domain

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

// MinimumPasswordLength is the minimum number of Unicode characters accepted
// for a password.
const MinimumPasswordLength = 12

// NormalizeEmail returns the canonical form used for email comparisons.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks an email address after normalizing surrounding
// whitespace and case. It returns the normalized address on success.
func ValidateEmail(email string) (string, error) {
	normalized := NormalizeEmail(email)
	if normalized == "" || !utf8.ValidString(normalized) || strings.ContainsAny(normalized, " \t\r\n") {
		return "", NewInvalid("invalid email")
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized || strings.Contains(normalized, "\"") {
		return "", NewInvalid("invalid email")
	}
	at := strings.LastIndexByte(normalized, '@')
	if strings.Count(normalized, "@") != 1 || at <= 0 || at == len(normalized)-1 {
		return "", NewInvalid("invalid email")
	}
	return normalized, nil
}

// ValidatePassword enforces the password length policy. The returned error
// intentionally contains no password-derived data.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < MinimumPasswordLength {
		return NewInvalid("password must be at least 12 characters")
	}
	return nil
}
