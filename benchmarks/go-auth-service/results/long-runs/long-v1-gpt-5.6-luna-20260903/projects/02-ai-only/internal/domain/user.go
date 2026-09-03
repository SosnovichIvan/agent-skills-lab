package domain

import "time"

// Email is kept as a distinct domain value so email handling does not get
// confused with arbitrary user-provided strings.
type Email string

// EmailVerificationState is deliberately explicit instead of inferring
// verification from the presence or absence of a timestamp.
type EmailVerificationState string

const (
	EmailUnverified EmailVerificationState = "unverified"
	EmailVerified   EmailVerificationState = "verified"
)

// PasswordMaterial contains only credential material and is never part of a
// public representation. The JSON exclusions also protect direct marshaling
// of this type if it is ever logged or transported accidentally.
type PasswordMaterial struct {
	Salt       []byte `json:"-"`
	Hash       []byte `json:"-"`
	Iterations int    `json:"-"`
	Algorithm  string `json:"-"`
}

// User is the private, persistence-facing user model. PasswordMaterial is
// intentionally excluded from JSON even when a User is marshaled directly.
type User struct {
	ID                UserID                 `json:"id"`
	Email             Email                  `json:"email"`
	EmailVerification EmailVerificationState `json:"email_verification"`
	PasswordMaterial  PasswordMaterial       `json:"-"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PublicProfile is the safe user shape returned by API-facing code. It has no
// field capable of carrying password material.
type PublicProfile struct {
	ID                UserID                 `json:"id"`
	Email             Email                  `json:"email"`
	EmailVerification EmailVerificationState `json:"email_verification"`
	CreatedAt         time.Time              `json:"created_at"`
}

func (u User) PublicProfile() PublicProfile {
	return PublicProfile{
		ID:                u.ID,
		Email:             u.Email,
		EmailVerification: u.EmailVerification,
		CreatedAt:         u.CreatedAt,
	}
}
