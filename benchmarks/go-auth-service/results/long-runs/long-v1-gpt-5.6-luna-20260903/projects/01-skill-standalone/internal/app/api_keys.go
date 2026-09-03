package app

import (
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
)

type createAPIKeyRequest struct {
	Name   string              `json:"name"`
	Scopes []domain.Permission `json:"scopes"`
}
type createAPIKeyResponse struct {
	APIKey domain.APIKey `json:"api_key"`
	Key    string        `json:"key"`
}

func (a *App) createAPIKey(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	var input createAPIKeyRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(input.Scopes) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "name and scopes are required", requestID)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	membership, err := a.memberships.Get(orgID, domain.UserID(claims.Subject))
	if err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "organization membership required", requestID)
		return
	}
	for _, scope := range input.Scopes {
		role, roleErr := a.roles.Get(orgID, domain.RoleID(membership.Role))
		if roleErr != nil || !role.HasPermission(scope) {
			writeError(w, http.StatusForbidden, "forbidden", "api key scope exceeds actor permissions", requestID)
			return
		}
	}
	id, err := domain.NewAPIKeyID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	secret, err := domain.NewAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	key := domain.APIKey{ID: id, OrganizationID: orgID, CreatedBy: domain.UserID(claims.Subject), Name: name, Scopes: input.Scopes, Hash: domain.HashAPIKey(secret), CreatedAt: now}
	if err := a.apiKeys.Create(key); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}
	a.recordAudit(r, orgID, "api_key.created", string(key.ID), map[string]any{"name": key.Name, "scopes": key.Scopes})
	key.Hash = nil
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: createAPIKeyResponse{APIKey: key, Key: secret}})
}

func (a *App) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys := a.apiKeys.ListByOrganization(domain.OrganizationID(r.PathValue("orgID")))
	for i := range keys {
		keys[i].Hash = nil
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: keys})
}

func (a *App) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	err := a.apiKeys.Revoke(domain.OrganizationID(r.PathValue("orgID")), domain.APIKeyID(r.PathValue("keyID")), a.clock.Now().UTC())
	if err != nil {
		status, code, message := http.StatusConflict, "conflict", "api key cannot be revoked"
		if domain.IsKind(err, domain.ErrNotFound) {
			status, code, message = http.StatusNotFound, "not_found", "api key not found"
		}
		writeError(w, status, code, message, requestID)
		return
	}
	a.recordAudit(r, domain.OrganizationID(r.PathValue("orgID")), "api_key.revoked", r.PathValue("keyID"), nil)
	w.WriteHeader(http.StatusNoContent)
}
