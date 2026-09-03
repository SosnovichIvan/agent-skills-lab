package domain

import "time"

// AuditEvent records a security-relevant state transition. Hash and
// PreviousHash form an append-only tamper-evident chain in the repository.
type AuditEvent struct {
	ID             ID             `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	ActorID        UserID         `json:"actor_id,omitempty"`
	Action         string         `json:"action"`
	Resource       string         `json:"resource,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
	PreviousHash   string         `json:"previous_hash,omitempty"`
	Hash           string         `json:"hash"`
}
