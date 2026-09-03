package app

import (
	"net/http"
	"strconv"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

func (a *App) appendAudit(organizationID, actorID domain.ID, action string, metadata map[string]string) error {
	id, err := a.IDGenerator.NewID()
	if err != nil {
		return err
	}
	return a.Audits.Append(domain.AuditEvent{
		ID: id, OrganizationID: organizationID, ActorID: actorID,
		Action: action, Metadata: metadata, CreatedAt: a.Clock.Now().UTC(),
	})
}

func (a *App) listAudit(w http.ResponseWriter, r *http.Request, organizationID domain.ID) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid audit pagination")
			return
		}
		limit = parsed
	}
	page, err := a.Audits.List(organizationID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid audit pagination")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]any{"events": page.Events, "next_cursor": page.NextCursor})
}
