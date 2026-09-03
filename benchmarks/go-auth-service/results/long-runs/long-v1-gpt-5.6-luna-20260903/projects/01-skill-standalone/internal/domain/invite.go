package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const InviteTokenBytes = 32

// Invite stores only the digest of the opaque token given to an invitee.
type Invite struct {
	ID             InviteID       `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	Email          string         `json:"email"`
	Role           string         `json:"role"`
	TokenHash      []byte         `json:"-"`
	InvitedBy      UserID         `json:"invited_by"`
	CreatedAt      time.Time      `json:"created_at"`
	ExpiresAt      time.Time      `json:"expires_at"`
	AcceptedAt     time.Time      `json:"accepted_at,omitempty"`
}

func NewInviteToken() (string, error) {
	var raw [InviteTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("generate invite token")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func HashInviteToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return append([]byte(nil), digest[:]...)
}

func ValidInviteToken(token string) bool {
	if strings.TrimSpace(token) != token || token == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == InviteTokenBytes
}
