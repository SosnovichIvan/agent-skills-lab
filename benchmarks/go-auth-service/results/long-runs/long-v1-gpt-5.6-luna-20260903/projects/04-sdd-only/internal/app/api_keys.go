package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type apiKeyCreateRequest struct {
	Name   string              `json:"name"`
	Scopes []domain.Permission `json:"scopes"`
}

func (a *App) createAPIKey(w http.ResponseWriter, r *http.Request, organizationID domain.ID) {
	claims, _ := httpapi.ClaimsFromContext(r.Context())
	var request apiKeyCreateRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	value, err := security.NewAPIKey()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	id, err := a.IDGenerator.NewID()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	key := domain.APIKey{ID: id, OrganizationID: organizationID, Name: request.Name, KeyHash: security.HashOpaqueToken(value), Scopes: request.Scopes, CreatedAt: a.Clock.Now().UTC()}
	if err := a.APIKeys.Create(key); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid API key")
		return
	}
	if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "apikey.created", map[string]string{"key_id": string(id)}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	response := struct {
		domain.APIKeyProfile
		Value string `json:"value"`
	}{APIKeyProfile: key.PublicProfile(), Value: value}
	httpapi.WriteData(w, http.StatusCreated, response)
}

func (a *App) listAPIKeys(w http.ResponseWriter, organizationID domain.ID) {
	keys := a.APIKeys.ListByOrganization(organizationID)
	profiles := make([]domain.APIKeyProfile, 0, len(keys))
	for _, key := range keys {
		profiles = append(profiles, key.PublicProfile())
	}
	httpapi.WriteData(w, http.StatusOK, profiles)
}

func (a *App) revokeAPIKey(w http.ResponseWriter, r *http.Request, organizationID, keyID domain.ID) {
	if err := a.APIKeys.Revoke(organizationID, keyID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "API key not found")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if claims, ok := httpapi.ClaimsFromContext(r.Context()); ok {
		if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "apikey.revoked", map[string]string{"key_id": string(keyID)}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"revoked": true})
}
