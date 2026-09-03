package domain

import "time"

type AuditEvent struct {
	Sequence       uint64         `json:"sequence"`
	OrganizationID OrganizationID `json:"organization_id"`
	ActorID        UserID         `json:"actor_id,omitempty"`
	Action         string         `json:"action"`
	TargetID       string         `json:"target_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	PreviousHash   string         `json:"previous_hash"`
	Hash           string         `json:"hash"`
}
