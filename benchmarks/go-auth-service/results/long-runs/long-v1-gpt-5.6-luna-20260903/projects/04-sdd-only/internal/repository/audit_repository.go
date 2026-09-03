package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type AuditPage struct {
	Events     []domain.AuditEvent
	NextCursor string
}

type AuditStore interface {
	Append(event domain.AuditEvent) error
	List(organizationID domain.ID, cursor string, limit int) (AuditPage, error)
}

type AuditRepository struct {
	mu     sync.RWMutex
	events []domain.AuditEvent
}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

func (r *AuditRepository) Append(event domain.AuditEvent) error {
	if event.ID == "" || event.OrganizationID == "" || event.ActorID == "" || event.Action == "" {
		return domain.NewError(domain.KindInvalid, "audit event is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	event.Metadata = cloneMetadata(event.Metadata)
	if len(r.events) > 0 {
		event.PreviousHash = r.events[len(r.events)-1].Hash
	}
	event.Hash = hashAuditEvent(event)
	r.events = append(r.events, cloneAuditEvent(event))
	return nil
}

func (r *AuditRepository) List(organizationID domain.ID, cursor string, limit int) (AuditPage, error) {
	if organizationID == "" {
		return AuditPage{}, domain.NewError(domain.KindInvalid, "organization ID is required")
	}
	if limit <= 0 || limit > 100 {
		return AuditPage{}, domain.NewError(domain.KindInvalid, "audit page limit is out of bounds")
	}
	start := 0
	if cursor != "" {
		var err error
		start, err = strconv.Atoi(cursor)
		if err != nil || start < 0 {
			return AuditPage{}, domain.NewError(domain.KindInvalid, "audit cursor is invalid")
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	filtered := make([]domain.AuditEvent, 0)
	for _, event := range r.events {
		if event.OrganizationID == organizationID {
			filtered = append(filtered, cloneAuditEvent(event))
		}
	}
	if start >= len(filtered) {
		return AuditPage{Events: []domain.AuditEvent{}}, nil
	}
	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := AuditPage{Events: filtered[start:end]}
	if end < len(filtered) {
		page.NextCursor = fmt.Sprintf("%d", end)
	}
	return page, nil
}

func hashAuditEvent(event domain.AuditEvent) string {
	canonical, _ := json.Marshal(struct {
		PreviousHash   string
		OrganizationID domain.ID
		ActorID        domain.ID
		Action         string
		Metadata       map[string]string
		CreatedAt      int64
	}{event.PreviousHash, event.OrganizationID, event.ActorID, event.Action, event.Metadata, event.CreatedAt.UnixNano()})
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func cloneAuditEvent(event domain.AuditEvent) domain.AuditEvent {
	event.Metadata = cloneMetadata(event.Metadata)
	return event
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[key] = value
	}
	return result
}
