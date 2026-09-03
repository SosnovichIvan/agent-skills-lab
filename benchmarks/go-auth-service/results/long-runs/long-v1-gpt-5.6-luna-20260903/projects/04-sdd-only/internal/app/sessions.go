package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request logoutRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.RefreshToken == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	session, err := a.Sessions.FindByTokenHash(security.HashOpaqueToken(request.RefreshToken))
	if err != nil || session.UserID != domain.ID(claims.Subject) {
		writeUnauthorized(w)
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil && !errors.Is(err, domain.ErrNotFound) {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (a *App) logoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	if err := a.Sessions.RevokeAllByUser(domain.ID(claims.Subject)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (a *App) sessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	stored := a.Sessions.ListByUser(domain.ID(claims.Subject))
	profiles := make([]domain.SessionProfile, 0, len(stored))
	for _, session := range stored {
		profiles = append(profiles, session.PublicProfile())
	}
	httpapi.WriteData(w, http.StatusOK, profiles)
}

func (a *App) sessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	const prefix = "/v1/sessions/"
	value := strings.TrimPrefix(r.URL.Path, prefix)
	if value == "" || strings.Contains(value, "/") {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	session, err := a.Sessions.Get(domain.ID(value))
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	if session.UserID != domain.ID(claims.Subject) {
		writeUnauthorized(w)
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"revoked": true})
}
