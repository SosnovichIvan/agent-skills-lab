package domain

import "time"

type AuditEvent struct {
	ID             ID                `json:"id"`
	OrganizationID ID                `json:"organization_id"`
	ActorID        ID                `json:"actor_id"`
	Action         string            `json:"action"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	PreviousHash   string            `json:"previous_hash"`
	Hash           string            `json:"hash"`
	CreatedAt      time.Time         `json:"created_at"`
}
