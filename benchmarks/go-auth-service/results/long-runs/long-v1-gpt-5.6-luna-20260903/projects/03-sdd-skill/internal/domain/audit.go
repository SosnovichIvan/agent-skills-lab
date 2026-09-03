package domain

import "time"

// AuditEvent is the public, non-secret record of a security mutation. Hash
// values are hex encoded so events can be inspected and verified without
// exposing credentials or request bodies.
type AuditEvent struct {
	ID             ID                `json:"id"`
	OrganizationID OrganizationID    `json:"organization_id"`
	ActorID        UserID            `json:"actor_id,omitempty"`
	Action         string            `json:"action"`
	Target         string            `json:"target,omitempty"`
	Details        map[string]string `json:"details,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	PreviousHash   string            `json:"previous_hash,omitempty"`
	Hash           string            `json:"hash"`
}
