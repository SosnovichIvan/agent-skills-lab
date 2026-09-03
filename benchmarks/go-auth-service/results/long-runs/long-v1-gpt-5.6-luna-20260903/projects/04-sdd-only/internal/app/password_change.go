package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *App) passwordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request passwordChangeRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	user, err := a.Users.Get(domain.ID(claims.Subject))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !security.VerifyPassword(request.CurrentPassword, user.PasswordMaterial) {
		writeUnauthorized(w)
		return
	}
	newMaterial, err := security.HashPassword(request.NewPassword)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid new password")
		return
	}
	user.PasswordMaterial = newMaterial
	if err := a.Users.Update(user); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Sessions.RevokeAllByUser(user.ID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"password_changed": true})
}
