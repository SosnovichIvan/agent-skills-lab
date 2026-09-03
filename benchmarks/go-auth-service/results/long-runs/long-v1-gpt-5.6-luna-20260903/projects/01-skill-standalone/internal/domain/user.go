package domain

import "time"

// EmailVerificationState describes the state of a user's email address.
// Keeping this state explicit avoids inferring verification from the presence
// or absence of unrelated user data.
type EmailVerificationState string

const (
	EmailVerificationPending  EmailVerificationState = "pending"
	EmailVerificationVerified EmailVerificationState = "verified"
)

// PasswordMaterial contains only the derived password material needed for
// password verification. It deliberately has no JSON representation: this
// type must not leak through API responses, logs, or other JSON envelopes.
type PasswordMaterial struct {
	Salt       []byte
	Hash       []byte
	Iterations uint32
}

// MarshalJSON prevents accidental serialization even when PasswordMaterial is
// marshaled directly rather than as part of User.
func (PasswordMaterial) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// User is the private domain representation of an account. PasswordMaterial
// is kept separate from the fields exposed by PublicUserProfile.
type User struct {
	ID                     UserID                 `json:"id"`
	Email                  string                 `json:"email"`
	EmailVerified          bool                   `json:"email_verified"`
	EmailVerificationState EmailVerificationState `json:"email_verification_state"`
	PasswordMaterial       PasswordMaterial       `json:"-"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

// PublicUserProfile is the safe representation of a user for API responses.
// It contains account identity and verification state, but no password data.
type PublicUserProfile struct {
	ID                     UserID                 `json:"id"`
	Email                  string                 `json:"email"`
	EmailVerified          bool                   `json:"email_verified"`
	EmailVerificationState EmailVerificationState `json:"email_verification_state"`
	CreatedAt              time.Time              `json:"created_at"`
}

// PublicProfile returns the user data that may be sent to a client.
func (u User) PublicProfile() PublicUserProfile {
	return PublicUserProfile{
		ID:                     u.ID,
		Email:                  u.Email,
		EmailVerified:          u.EmailVerified,
		EmailVerificationState: u.EmailVerificationState,
		CreatedAt:              u.CreatedAt,
	}
}
