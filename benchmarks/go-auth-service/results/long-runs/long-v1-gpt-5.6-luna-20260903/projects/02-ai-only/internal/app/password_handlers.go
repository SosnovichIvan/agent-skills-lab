package app

import (
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	var request passwordChangeRequest
	if err := decodeJSON(r, &request); err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	user, err := a.Users.Get(domain.UserID(claims.Subject))
	if err != nil || !security.VerifyPassword(user.PasswordMaterial, request.CurrentPassword) {
		writeUnauthorized(w, r)
		return
	}
	newMaterial, err := security.HashPassword(request.NewPassword)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_password", "password does not meet policy")
		return
	}
	user.PasswordMaterial = newMaterial
	user.UpdatedAt = a.Clock.Now().UTC()
	if err := a.Users.Update(user); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to change password")
		return
	}
	a.Sessions.RevokeAllByUser(user.ID)
	a.audit("", user.ID, "password.changed", string(user.ID))
	writeData(w, http.StatusOK, user.PublicProfile())
}
