package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

const InviteTokenSize = 32

// InviteToken is the 6.3 invite model. It is persisted server-side; only the
// digest is stored and the opaque bearer is never a field on this type.
type InviteToken struct {
	TokenHash      []byte         `json:"-"`
	OrganizationID OrganizationID `json:"organization_id"`
	Email          string         `json:"email"`
	Role           string         `json:"role"`
	ExpiresAt      time.Time      `json:"expires_at"`
	Used           bool           `json:"-"`
}

// MarshalJSON deliberately omits persistence and consumption state even if a
// repository value is accidentally passed to a response encoder.
func (i InviteToken) MarshalJSON() ([]byte, error) {
	type publicInvite struct {
		OrganizationID OrganizationID `json:"organization_id"`
		Email          string         `json:"email"`
		Role           string         `json:"role"`
		ExpiresAt      time.Time      `json:"expires_at"`
	}
	return json.Marshal(publicInvite{
		OrganizationID: i.OrganizationID,
		Email:          i.Email,
		Role:           i.Role,
		ExpiresAt:      i.ExpiresAt,
	})
}

// Clone returns a detached invite snapshot. In particular, callers cannot
// mutate the stored digest through a returned value.
func (i InviteToken) Clone() InviteToken {
	i.TokenHash = append([]byte(nil), i.TokenHash...)
	return i
}

func NewInviteToken() (string, error) {
	value := make([]byte, InviteTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", NewError(ErrorInvalid, "could not create invite token")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func HashInviteToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}
