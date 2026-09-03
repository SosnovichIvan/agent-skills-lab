package domain

import "time"

type RoleAssignment struct {
	OrganizationID OrganizationID `json:"organization_id"`
	UserID         UserID         `json:"user_id"`
	RoleID         RoleID         `json:"role_id"`
	AssignedAt     time.Time      `json:"assigned_at"`
}
