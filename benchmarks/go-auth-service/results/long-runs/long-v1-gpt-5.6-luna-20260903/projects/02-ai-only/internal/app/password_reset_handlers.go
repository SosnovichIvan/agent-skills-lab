package app

import (
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (a *App) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	var request passwordResetRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	resetToken, err := security.NewPasswordResetToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create password reset request")
		return
	}
	email, _ := domain.NormalizeEmail(request.Email)
	user, err := a.Users.FindByEmail(domain.Email(email))
	if err == nil {
		now := a.Clock.Now().UTC()
		resetID, idErr := domain.NewID()
		if idErr == nil {
			_ = a.Resets.Create(domain.PasswordReset{
				ID: resetID, UserID: user.ID,
				TokenHash: security.HashPasswordResetToken(resetToken),
				ExpiresAt: now.Add(a.Config.RefreshTokenTTL), CreatedAt: now,
			})
		}
	}
	// The same status, shape, and generic message are returned for every
	// email, while the opaque token keeps this in-memory flow usable.
	writeData(w, http.StatusAccepted, map[string]string{
		"message":     "if the account exists, a password reset was requested",
		"reset_token": resetToken,
	})
}

func (a *App) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	var request passwordResetConfirmRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	if request.Token == "" {
		writeUnauthorized(w, r)
		return
	}
	newMaterial, err := security.HashPassword(request.NewPassword)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_password", "password does not meet policy")
		return
	}
	reset, err := a.Resets.Consume(security.HashPasswordResetToken(request.Token))
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	if !a.Clock.Now().UTC().Before(reset.ExpiresAt) {
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.Get(reset.UserID)
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	user.PasswordMaterial = newMaterial
	user.UpdatedAt = a.Clock.Now().UTC()
	if err := a.Users.Update(user); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to reset password")
		return
	}
	a.Sessions.RevokeAllByUser(user.ID)
	a.audit("", user.ID, "password.reset", string(user.ID))
	writeData(w, http.StatusOK, user.PublicProfile())
}
