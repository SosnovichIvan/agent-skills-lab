package domain

import "time"

type APIKey struct {
	ID             APIKeyID            `json:"id"`
	OrganizationID OrganizationID      `json:"organization_id"`
	OwnerID        UserID              `json:"owner_id"`
	Name           string              `json:"name"`
	KeyHash        []byte              `json:"-"`
	Scopes         map[Permission]bool `json:"scopes"`
	Revoked        bool                `json:"revoked"`
	CreatedAt      time.Time           `json:"created_at"`
}
