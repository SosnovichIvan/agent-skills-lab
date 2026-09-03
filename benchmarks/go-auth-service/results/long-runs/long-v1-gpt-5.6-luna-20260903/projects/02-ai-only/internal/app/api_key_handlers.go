package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type createAPIKeyRequest struct {
	Name   string              `json:"name"`
	Scopes []domain.Permission `json:"scopes"`
}

type apiKeyView struct {
	ID             domain.APIKeyID            `json:"id"`
	OrganizationID domain.OrganizationID      `json:"organization_id"`
	OwnerID        domain.UserID              `json:"owner_id"`
	Name           string                     `json:"name"`
	Scopes         map[domain.Permission]bool `json:"scopes"`
	Revoked        bool                       `json:"revoked"`
	CreatedAt      string                     `json:"created_at"`
}

func (a *App) createAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	ownerID, ok := a.principalUserID(r)
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	var request createAPIKeyRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Name) == "" || len(request.Scopes) == 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "API key request is invalid")
		return
	}
	for _, scope := range request.Scopes {
		if !a.hasPermission(orgID, ownerID, scope) {
			if key, isKey := APIKeyFromContext(r.Context()); !isKey || key.OrganizationID != orgID || !key.Scopes[scope] {
				writeUnauthorized(w, r)
				return
			}
		}
	}
	rawKey, err := security.NewAPIKey()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create API key")
		return
	}
	scopes := make(map[domain.Permission]bool, len(request.Scopes))
	for _, scope := range request.Scopes {
		if scope == "" {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "API key scope is invalid")
			return
		}
		scopes[scope] = true
	}
	id, err := domain.NewAPIKeyID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create API key")
		return
	}
	key := domain.APIKey{ID: id, OrganizationID: orgID, OwnerID: ownerID, Name: strings.TrimSpace(request.Name), KeyHash: security.HashAPIKey(rawKey), Scopes: scopes, CreatedAt: a.Clock.Now().UTC()}
	if err := a.APIKeys.Create(key); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create API key")
		return
	}
	a.audit(orgID, ownerID, "apikey.created", string(key.ID))
	writeData(w, http.StatusCreated, map[string]any{"id": key.ID, "name": key.Name, "scopes": key.Scopes, "api_key": rawKey})
}

func (a *App) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	keys := a.APIKeys.ListByOrganization(orgID)
	views := make([]apiKeyView, 0, len(keys))
	for _, key := range keys {
		views = append(views, apiKeyView{ID: key.ID, OrganizationID: key.OrganizationID, OwnerID: key.OwnerID, Name: key.Name, Scopes: key.Scopes, Revoked: key.Revoked, CreatedAt: key.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
	}
	writeData(w, http.StatusOK, views)
}

func (a *App) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	key, err := a.APIKeys.Get(domain.APIKeyID(r.PathValue("keyID")))
	if err != nil || key.OrganizationID != orgID {
		writeError(w, r, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	if err := a.APIKeys.Revoke(key.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "not_found", "API key not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to revoke API key")
		return
	}
	if ownerID, ok := a.principalUserID(r); ok {
		a.audit(orgID, ownerID, "apikey.revoked", string(key.ID))
	}
	writeData(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *App) principalUserID(r *http.Request) (domain.UserID, bool) {
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		return domain.UserID(claims.Subject), true
	}
	if key, ok := APIKeyFromContext(r.Context()); ok {
		return key.OwnerID, true
	}
	return "", false
}
