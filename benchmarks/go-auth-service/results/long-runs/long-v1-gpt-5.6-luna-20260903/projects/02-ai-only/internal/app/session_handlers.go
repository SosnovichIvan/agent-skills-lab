package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type sessionView struct {
	ID        domain.SessionID `json:"id"`
	ExpiresAt string           `json:"expires_at"`
	FamilyID  domain.ID        `json:"family_id"`
	Revoked   bool             `json:"revoked"`
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	var request refreshRequest
	if err := decodeJSON(r, &request); err != nil || request.RefreshToken == "" {
		writeUnauthorized(w, r)
		return
	}
	session, err := a.Sessions.FindByRefreshTokenHash(security.HashRefreshToken(request.RefreshToken))
	if err != nil || session.UserID != domain.UserID(claims.Subject) {
		writeUnauthorized(w, r)
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to revoke session")
		return
	}
	a.audit("", domain.UserID(claims.Subject), "session.revoked", string(session.ID))
	writeData(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *App) logoutAll(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	count := a.Sessions.RevokeAllByUser(domain.UserID(claims.Subject))
	a.audit("", domain.UserID(claims.Subject), "sessions.revoked_all", "")
	writeData(w, http.StatusOK, map[string]int{"revoked": count})
}

func (a *App) sessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	stored := a.Sessions.ListByUser(domain.UserID(claims.Subject))
	views := make([]sessionView, 0, len(stored))
	for _, session := range stored {
		views = append(views, sessionView{
			ID: session.ID, ExpiresAt: session.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			FamilyID: session.FamilyID, Revoked: session.Revoked,
		})
	}
	writeData(w, http.StatusOK, views)
}

func (a *App) deleteSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	sessionID := domain.SessionID(r.PathValue("sessionID"))
	session, err := a.Sessions.Get(sessionID)
	if err != nil || session.UserID != domain.UserID(claims.Subject) {
		writeUnauthorized(w, r)
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to revoke session")
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"revoked": true})
}
