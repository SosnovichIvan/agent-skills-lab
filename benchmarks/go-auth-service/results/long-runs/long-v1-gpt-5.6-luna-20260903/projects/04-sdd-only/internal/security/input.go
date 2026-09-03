// Package security contains authentication input policies and primitives.
package security

import (
	"net/mail"
	"strings"
	"unicode/utf8"

	"benchmark.local/iam/internal/domain"
)

const MinPasswordLength = 12

// NormalizeEmail trims surrounding whitespace, lowercases the address, and
// rejects values that are not a single plain email address. The returned
// error never contains the supplied address.
func NormalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", domain.NewError(domain.KindInvalid, "email is invalid")
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized || strings.Count(normalized, "@") != 1 {
		return "", domain.NewError(domain.KindInvalid, "email is invalid")
	}
	local, domainPart, ok := strings.Cut(normalized, "@")
	if !ok || local == "" || domainPart == "" || strings.ContainsAny(normalized, "\r\n") {
		return "", domain.NewError(domain.KindInvalid, "email is invalid")
	}
	return normalized, nil
}

// ValidatePassword enforces the password length policy without echoing or
// embedding password material in the returned error.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < MinPasswordLength {
		return domain.NewError(domain.KindInvalid, "password does not meet the minimum length")
	}
	return nil
}
