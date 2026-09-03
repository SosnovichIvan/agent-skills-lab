package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request refreshRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	if request.RefreshToken == "" {
		writeUnauthorized(w)
		return
	}
	oldHash := security.HashOpaqueToken(request.RefreshToken)
	oldSession, err := a.Sessions.FindByTokenHash(oldHash)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	now := a.Clock.Now().UTC()
	if oldSession.Revoked {
		// A revoked token is a previously rotated token. Reuse detection
		// invalidates every session in its family.
		_ = a.Sessions.RevokeFamily(oldSession.FamilyID)
		writeUnauthorized(w)
		return
	}
	if !now.Before(oldSession.ExpiresAt) {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.Get(oldSession.UserID)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	newRefreshToken, replacement, err := a.newSession(user.ID, oldSession.FamilyID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Sessions.Rotate(oldHash, replacement); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	accessToken, err := a.AccessToken.Issue(user)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
	})
}

func (a *App) createSession(userID domain.ID, familyID domain.ID) (string, error) {
	refreshToken, session, err := a.newSession(userID, familyID)
	if err != nil {
		return "", err
	}
	if err := a.Sessions.Create(session); err != nil {
		return "", err
	}
	return refreshToken, nil
}

func (a *App) newSession(userID domain.ID, familyID domain.ID) (string, domain.Session, error) {
	if familyID == "" {
		var err error
		familyID, err = a.IDGenerator.NewID()
		if err != nil {
			return "", domain.Session{}, err
		}
	}
	sessionID, err := a.IDGenerator.NewID()
	if err != nil {
		return "", domain.Session{}, err
	}
	refreshToken, err := security.NewRefreshToken()
	if err != nil {
		return "", domain.Session{}, err
	}
	now := a.Clock.Now().UTC()
	return refreshToken, domain.Session{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: security.HashOpaqueToken(refreshToken),
		FamilyID:  familyID,
		ExpiresAt: now.Add(a.Config.RefreshTokenTTL),
	}, nil
}
