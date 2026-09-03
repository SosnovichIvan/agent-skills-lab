package domain

import "time"

// Organization is a tenant and always has an owner account.
type Organization struct {
	ID        OrganizationID `json:"id"`
	Name      string         `json:"name"`
	OwnerID   UserID         `json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Membership connects one user to one organization. Role details are resolved
// by the IAM layer from the organization-scoped role repository.
type Membership struct {
	ID             ID             `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	UserID         UserID         `json:"user_id"`
	Role           string         `json:"role"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
