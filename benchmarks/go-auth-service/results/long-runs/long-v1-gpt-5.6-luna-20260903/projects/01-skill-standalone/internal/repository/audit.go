package repository

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sync"

	"benchmark.local/iam/internal/domain"
)

// AuditRepository is an append-only, concurrency-safe hash-chain store.
type AuditRepository struct {
	mu     sync.RWMutex
	events []domain.AuditEvent
}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

func (r *AuditRepository) Append(event domain.AuditEvent) (domain.AuditEvent, error) {
	if event.ID == "" || event.OrganizationID == "" || event.Action == "" || event.OccurredAt.IsZero() {
		return domain.AuditEvent{}, domain.NewInvalid("invalid audit event")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) > 0 {
		event.PreviousHash = r.events[len(r.events)-1].Hash
	}
	// encoding/json sorts map keys, giving the digest a deterministic form.
	payload, err := json.Marshal(struct {
		ID, OrganizationID, ActorID, Action, Resource, PreviousHash string
		Metadata                                                    map[string]any
		OccurredAt                                                  any
	}{string(event.ID), string(event.OrganizationID), string(event.ActorID), event.Action, event.Resource, event.PreviousHash, event.Metadata, event.OccurredAt.UTC()})
	if err != nil {
		return domain.AuditEvent{}, err
	}
	sum := sha256.Sum256(payload)
	event.Hash = hex.EncodeToString(sum[:])
	event.Metadata = cloneMetadata(event.Metadata)
	r.events = append(r.events, event)
	return cloneAuditEvent(event), nil
}

func cloneMetadata(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func cloneAuditEvent(e domain.AuditEvent) domain.AuditEvent {
	e.Metadata = cloneMetadata(e.Metadata)
	return e
}

// ListByOrganization returns at most limit events and an opaque cursor for the
// next page. Cursor pagination is bounded to prevent unbounded responses.
func (r *AuditRepository) ListByOrganization(orgID domain.OrganizationID, cursor string, limit int) ([]domain.AuditEvent, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", domain.NewInvalid("limit must be between 1 and 100")
	}
	start := 0
	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", domain.NewInvalid("invalid cursor")
		}
		parts := string(decoded)
		prefix := string(orgID) + ":"
		if len(parts) <= len(prefix) || parts[:len(prefix)] != prefix {
			return nil, "", domain.NewInvalid("invalid cursor")
		}
		id := domain.ID(parts[len(prefix):])
		r.mu.RLock()
		found := false
		for i, e := range r.events {
			if e.ID == id {
				start = i + 1
				found = true
				break
			}
		}
		r.mu.RUnlock()
		if !found {
			return nil, "", domain.NewInvalid("invalid cursor")
		}
	}
	r.mu.RLock()
	filtered := make([]domain.AuditEvent, 0)
	for i := start; i < len(r.events); i++ {
		if r.events[i].OrganizationID == orgID {
			filtered = append(filtered, cloneAuditEvent(r.events[i]))
		}
	}
	r.mu.RUnlock()
	if len(filtered) <= limit {
		return filtered, "", nil
	}
	page := filtered[:limit]
	next := base64.RawURLEncoding.EncodeToString([]byte(string(orgID) + ":" + string(page[len(page)-1].ID)))
	return page, next, nil
}

func (r *AuditRepository) Verify() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var previous string
	for _, event := range r.events {
		payload, err := json.Marshal(struct {
			ID, OrganizationID, ActorID, Action, Resource, PreviousHash string
			Metadata                                                    map[string]any
			OccurredAt                                                  any
		}{string(event.ID), string(event.OrganizationID), string(event.ActorID), event.Action, event.Resource, event.PreviousHash, event.Metadata, event.OccurredAt.UTC()})
		if err != nil {
			return false
		}
		sum := sha256.Sum256(payload)
		if event.PreviousHash != previous || event.Hash != hex.EncodeToString(sum[:]) {
			return false
		}
		previous = event.Hash
	}
	return true
}
