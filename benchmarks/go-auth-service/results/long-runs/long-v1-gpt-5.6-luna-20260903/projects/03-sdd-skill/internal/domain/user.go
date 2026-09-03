package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const MinimumPasswordLength = 12

// NormalizeEmail trims surrounding whitespace and applies the canonical
// lower-case representation used for identity lookups.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail returns a normalized email address or a safe validation
// error. The original address is never included in the error message.
func ValidateEmail(email string) (string, error) {
	normalized := NormalizeEmail(email)
	if normalized == "" || strings.ContainsAny(normalized, "\r\n") {
		return "", NewError(ErrorInvalid, "invalid email address")
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", NewError(ErrorInvalid, "invalid email address")
	}
	return normalized, nil
}

// ValidatePassword enforces the minimum password length without ever
// including password material in the returned error.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinimumPasswordLength {
		return NewError(ErrorInvalid, "password does not meet the minimum length")
	}
	return nil
}

// PasswordMaterial contains the derived password representation used by the
// password verifier. It deliberately has no JSON representation: password
// material is an internal authentication detail, not part of an API model.
//
// The byte slices are copied by Clone so callers do not accidentally share
// mutable credential state when a user is copied.
type PasswordMaterial struct {
	Salt       []byte `json:"-"`
	Hash       []byte `json:"-"`
	Iterations uint32 `json:"-"`
}

// Clone returns an independent copy of the password material.
func (p PasswordMaterial) Clone() PasswordMaterial {
	return PasswordMaterial{
		Salt:       append([]byte(nil), p.Salt...),
		Hash:       append([]byte(nil), p.Hash...),
		Iterations: p.Iterations,
	}
}

// PublicProfile is the safe, externally visible part of a user account.
// Password material is intentionally not present in this type.
type PublicProfile struct {
	ID            UserID    `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// User is the complete internal user aggregate. Authentication material is
// kept separate from the public account profile and is excluded from JSON.
type User struct {
	ID               UserID           `json:"id"`
	Email            string           `json:"email"`
	EmailVerified    bool             `json:"email_verified"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	PasswordMaterial PasswordMaterial `json:"-"`
}

// PublicProfile returns a detached profile suitable for API responses.
func (u User) PublicProfile() PublicProfile {
	return PublicProfile{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}
