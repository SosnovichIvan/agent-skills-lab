package domain

import "time"

// Organization is a tenant and always has an owning user.
type Organization struct {
	ID        ID        `json:"id"`
	Name      string    `json:"name"`
	OwnerID   ID        `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Membership links one user to one organization.
type Membership struct {
	OrganizationID ID        `json:"organization_id"`
	UserID         ID        `json:"user_id"`
	Role           string    `json:"role"`
	RoleIDs        []ID      `json:"role_ids,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

const (
	OwnerRole  = "owner"
	AdminRole  = "admin"
	ViewerRole = "viewer"
)
