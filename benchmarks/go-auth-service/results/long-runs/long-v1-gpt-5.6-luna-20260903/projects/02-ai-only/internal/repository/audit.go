package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type AuditRepository struct {
	mu     sync.RWMutex
	next   uint64
	last   []byte
	events []domain.AuditEvent
}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

func (r *AuditRepository) Append(event domain.AuditEvent) domain.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	event.Sequence = r.next
	event.PreviousHash = hex.EncodeToString(r.last)
	// Only redacted, structured fields enter the chain; secrets cannot be
	// accidentally included through free-form event data.
	canonical, _ := json.Marshal(struct {
		Sequence uint64
		Org      domain.OrganizationID
		Actor    domain.UserID
		Action   string
		Target   string
		Created  string
		Previous string
	}{event.Sequence, event.OrganizationID, event.ActorID, event.Action, event.TargetID, event.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), event.PreviousHash})
	hash := sha256.Sum256(canonical)
	event.Hash = hex.EncodeToString(hash[:])
	r.last = append(r.last[:0], hash[:]...)
	r.events = append(r.events, event)
	return event
}

func (r *AuditRepository) List(organizationID domain.OrganizationID, after uint64, limit int) ([]domain.AuditEvent, uint64) {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.AuditEvent, 0, limit)
	for _, event := range r.events {
		if event.OrganizationID != organizationID || event.Sequence <= after {
			continue
		}
		result = append(result, event)
		if len(result) == limit {
			break
		}
	}
	var next uint64
	if len(result) == limit {
		next = result[len(result)-1].Sequence
	}
	return result, next
}
