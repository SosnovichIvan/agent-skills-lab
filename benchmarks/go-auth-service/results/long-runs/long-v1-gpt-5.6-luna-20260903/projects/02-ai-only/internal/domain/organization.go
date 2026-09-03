package domain

import "time"

type Organization struct {
	ID        OrganizationID `json:"id"`
	Name      string         `json:"name"`
	OwnerID   UserID         `json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Membership struct {
	OrganizationID OrganizationID `json:"organization_id"`
	UserID         UserID         `json:"user_id"`
	Role           string         `json:"role"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
