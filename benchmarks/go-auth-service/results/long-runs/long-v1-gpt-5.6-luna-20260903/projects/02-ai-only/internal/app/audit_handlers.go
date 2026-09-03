package app

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"benchmark.local/iam/internal/domain"
)

func (a *App) audit(orgID domain.OrganizationID, actor domain.UserID, action, target string) {
	a.Audit.Append(domain.AuditEvent{OrganizationID: orgID, ActorID: actor, Action: action, TargetID: target, CreatedAt: a.Clock.Now().UTC()})
}

func (a *App) listAudit(w http.ResponseWriter, r *http.Request) {
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "audit limit is invalid")
			return
		}
		limit = parsed
	}
	var after uint64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "audit cursor is invalid")
			return
		}
		after, err = strconv.ParseUint(string(decoded), 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "audit cursor is invalid")
			return
		}
	}
	events, next := a.Audit.List(orgID, after, limit)
	response := map[string]any{"items": events}
	if next != 0 {
		response["next_cursor"] = base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatUint(next, 10)))
	}
	writeData(w, http.StatusOK, response)
}
