package domain

import "time"

type Invite struct {
	ID             InviteID       `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	InvitedEmail   Email          `json:"invited_email"`
	Role           string         `json:"role"`
	TokenHash      []byte         `json:"-"`
	ExpiresAt      time.Time      `json:"expires_at"`
	Used           bool           `json:"used"`
	CreatedBy      UserID         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
}
