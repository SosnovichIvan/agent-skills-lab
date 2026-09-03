package app

import (
	"net/http"
	"strconv"
	"strings"

	"benchmark.local/iam/internal/domain"
)

type auditPage struct {
	Events     []domain.AuditEvent `json:"events"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

func (a *App) recordAudit(r *http.Request, orgID domain.OrganizationID, action, resource string, metadata map[string]any) {
	if a.audit == nil {
		return
	}
	claims, _ := ClaimsFromContext(r.Context())
	actor := domain.UserID(claims.Subject)
	id, err := domain.NewID()
	if err != nil {
		return
	}
	// Metadata is deliberately supplied by trusted handlers only. These keys
	// are stripped as a final defense against accidentally recording secrets.
	for _, key := range []string{"password", "token", "refresh_token", "access_token", "api_key", "key", "authorization", "secret"} {
		delete(metadata, key)
	}
	_, _ = a.audit.Append(domain.AuditEvent{ID: id, OrganizationID: orgID, ActorID: actor, Action: action, Resource: resource, Metadata: metadata, OccurredAt: a.clock.Now().UTC()})
}

func (a *App) recordUserAudit(r *http.Request, userID domain.UserID, action string) {
	for _, membership := range a.memberships.ListByUser(userID) {
		a.recordAudit(r, membership.OrganizationID, action, string(userID), nil)
	}
}

func (a *App) listAudit(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, _, ok := a.organizationMember(r)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "organization membership required", requestID)
		return
	}
	limit := 50
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100", requestID)
			return
		}
		limit = parsed
	}
	events, next, err := a.audit.ListByOrganization(orgID, strings.TrimSpace(r.URL.Query().Get("cursor")), limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid audit cursor", requestID)
		return
	}
	for i := range events {
		redactAuditMetadata(events[i].Metadata)
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: auditPage{Events: events, NextCursor: next}})
}

func redactAuditMetadata(metadata map[string]any) {
	for key := range metadata {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || lower == "key" || strings.Contains(lower, "api_key") {
			metadata[key] = "[REDACTED]"
		}
	}
}
