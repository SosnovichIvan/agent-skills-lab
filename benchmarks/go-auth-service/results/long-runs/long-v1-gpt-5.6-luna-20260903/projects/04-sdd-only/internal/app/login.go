package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request loginRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	email, err := security.NormalizeEmail(request.Email)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	now := a.Clock.Now().UTC()
	if a.LoginGuard.Locked(email, now) {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			a.LoginGuard.RecordFailure(email, now)
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !security.VerifyPassword(request.Password, user.PasswordMaterial) {
		a.LoginGuard.RecordFailure(email, now)
		writeUnauthorized(w)
		return
	}
	a.LoginGuard.RecordSuccess(email)
	accessToken, err := a.AccessToken.Issue(user)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	refreshToken, err := a.createSession(user.ID, "")
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
}
