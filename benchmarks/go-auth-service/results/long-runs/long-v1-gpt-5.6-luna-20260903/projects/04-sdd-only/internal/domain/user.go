package domain

import (
	"time"
)

// VerificationState is the lifecycle state of a user's email address.
type VerificationState string

const (
	VerificationPending  VerificationState = "pending"
	VerificationVerified VerificationState = "verified"
)

// PasswordMaterial contains only derived password data. Every field is
// excluded from JSON so accidental serialization of a User cannot disclose
// password material.
type PasswordMaterial struct {
	Salt       []byte `json:"-"`
	Hash       []byte `json:"-"`
	Iterations int    `json:"-"`
}

// User is the private domain representation of an account. PasswordMaterial
// is deliberately separate from the public profile returned by HTTP APIs.
type User struct {
	ID                ID                `json:"id"`
	Email             string            `json:"email"`
	PasswordMaterial  PasswordMaterial  `json:"-"`
	EmailVerification VerificationState `json:"email_verification"`
	CreatedAt         time.Time         `json:"created_at"`
}

// PublicProfile is the only user representation intended for API responses.
// It contains no password-derived fields or other authentication material.
type PublicProfile struct {
	ID            ID        `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// PublicProfile returns a safe projection of the user.
func (u User) PublicProfile() PublicProfile {
	return PublicProfile{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerification == VerificationVerified,
		CreatedAt:     u.CreatedAt,
	}
}
