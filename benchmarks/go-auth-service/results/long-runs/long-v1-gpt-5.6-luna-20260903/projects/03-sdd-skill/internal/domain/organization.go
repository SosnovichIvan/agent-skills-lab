package domain

import "time"

// Organization is a tenant and records the user who currently owns it.
type Organization struct {
	ID        OrganizationID `json:"id"`
	Name      string         `json:"name"`
	OwnerID   UserID         `json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Membership associates one user with one organization. Repositories enforce
// that an organization/user pair can occur at most once.
type Membership struct {
	ID             MembershipID   `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	UserID         UserID         `json:"user_id"`
	Role           string         `json:"role"`
	RoleIDs        []RoleID       `json:"role_ids,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
