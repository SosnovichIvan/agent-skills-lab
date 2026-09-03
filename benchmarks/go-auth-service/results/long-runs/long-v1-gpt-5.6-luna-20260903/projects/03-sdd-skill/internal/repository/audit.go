package repository

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"

	"benchmark.local/iam/internal/domain"
)

// AuditRepository stores append-only audit events. The mutex makes append and
// pagination safe, while the hash of each event commits to its predecessor.
type AuditRepository struct {
	mu     sync.RWMutex
	events []domain.AuditEvent
}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

type auditHashInput struct {
	ID, OrganizationID, ActorID, Action, Target string
	Details                                     map[string]string
	CreatedAt                                   string
	PreviousHash                                string
}

func eventHash(event domain.AuditEvent) string {
	data, _ := json.Marshal(auditHashInput{string(event.ID), string(event.OrganizationID), string(event.ActorID), event.Action, event.Target, event.Details, event.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), event.PreviousHash})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneAuditEvent(event domain.AuditEvent) domain.AuditEvent {
	event.Details = cloneDetails(event.Details)
	return event
}

func cloneDetails(details map[string]string) map[string]string {
	if details == nil {
		return nil
	}
	copyOf := make(map[string]string, len(details))
	for key, value := range details {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || lower == "key" || strings.Contains(lower, "refresh") {
			continue
		}
		copyOf[key] = value
	}
	return copyOf
}

// Append accepts only safe, non-secret event fields and computes chain values.
func (r *AuditRepository) Append(event domain.AuditEvent) (domain.AuditEvent, error) {
	if r == nil || event.OrganizationID == "" || event.Action == "" || event.CreatedAt.IsZero() {
		return domain.AuditEvent{}, domain.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return domain.AuditEvent{}, err
		}
		event.ID = id
	}
	if len(r.events) > 0 {
		event.PreviousHash = r.events[len(r.events)-1].Hash
	}
	event.Details = cloneDetails(event.Details)
	event.Hash = eventHash(event)
	r.events = append(r.events, cloneAuditEvent(event))
	return cloneAuditEvent(event), nil
}

type auditCursor struct{ Offset, Bound int }

func encodeAuditCursor(cursor auditCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeAuditCursor(raw string) (auditCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return auditCursor{}, domain.ErrInvalid
	}
	var cursor auditCursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Offset < 0 || cursor.Bound < 0 {
		return auditCursor{}, domain.ErrInvalid
	}
	return cursor, nil
}

// ListByOrganization returns at most limit events and an opaque cursor. The
// cursor bounds the result to the collection size observed on the first page.
func (r *AuditRepository) ListByOrganization(orgID domain.OrganizationID, cursor string, limit int) ([]domain.AuditEvent, string, error) {
	if r == nil || orgID == "" || limit <= 0 {
		return nil, "", domain.ErrInvalid
	}
	if limit > 100 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := auditCursor{Bound: len(r.events)}
	if cursor != "" {
		var err error
		state, err = decodeAuditCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		if state.Bound > len(r.events) {
			state.Bound = len(r.events)
		}
	}
	if state.Offset > state.Bound {
		return nil, "", domain.ErrInvalid
	}
	result := make([]domain.AuditEvent, 0, limit)
	for i := state.Offset; i < state.Bound && len(result) < limit; i++ {
		if r.events[i].OrganizationID == orgID {
			result = append(result, cloneAuditEvent(r.events[i]))
		}
	}
	// Cursor advances through the global append-only stream, not just tenant
	// events, so it cannot loop when events for other tenants are interleaved.
	position := state.Offset
	for position < state.Bound && len(result) > 0 {
		if r.events[position].OrganizationID == orgID {
			break
		}
		position++
	}
	seen := 0
	for i := state.Offset; i < state.Bound; i++ {
		if r.events[i].OrganizationID == orgID {
			seen++
			if seen == len(result) {
				position = i + 1
				break
			}
		}
	}
	if position < state.Bound {
		for i := position; i < state.Bound; i++ {
			if r.events[i].OrganizationID == orgID {
				return result, encodeAuditCursor(auditCursor{Offset: position, Bound: state.Bound}), nil
			}
		}
	}
	return result, "", nil
}

// Verify checks the complete chain and is useful for operational integrity checks.
func (r *AuditRepository) Verify() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	previous := ""
	for _, event := range r.events {
		if event.PreviousHash != previous || event.Hash != eventHash(event) {
			return false
		}
		previous = event.Hash
	}
	return true
}

// Len is intentionally small and read-only, useful for health checks.
func (r *AuditRepository) Len() int { r.mu.RLock(); defer r.mu.RUnlock(); return len(r.events) }
