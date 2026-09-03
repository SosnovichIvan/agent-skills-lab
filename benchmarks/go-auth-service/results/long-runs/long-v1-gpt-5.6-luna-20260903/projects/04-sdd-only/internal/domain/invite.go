package domain

import "time"

// Invite stores only the digest of the invitation token.
type Invite struct {
	ID             ID        `json:"id"`
	OrganizationID ID        `json:"organization_id"`
	InvitedEmail   string    `json:"invited_email"`
	TokenHash      []byte    `json:"-"`
	ExpiresAt      time.Time `json:"expires_at"`
	Used           bool      `json:"used"`
}
