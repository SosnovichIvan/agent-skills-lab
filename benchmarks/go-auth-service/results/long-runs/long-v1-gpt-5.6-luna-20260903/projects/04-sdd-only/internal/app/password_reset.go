package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type passwordResetRequestBody struct {
	Email string `json:"email"`
}

type passwordResetConfirmBody struct {
	ResetToken  string `json:"reset_token"`
	NewPassword string `json:"new_password"`
}

func (a *App) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request passwordResetRequestBody
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	resetToken, err := security.NewPasswordResetToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	email, normalizeErr := security.NormalizeEmail(request.Email)
	if normalizeErr == nil {
		if user, findErr := a.Users.FindByEmail(email); findErr == nil {
			id, idErr := a.IDGenerator.NewID()
			if idErr != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			token := domain.PasswordResetToken{
				ID: id, UserID: user.ID,
				TokenHash: security.HashOpaqueToken(resetToken),
				ExpiresAt: a.Clock.Now().UTC().Add(a.Config.PasswordResetTTL),
			}
			if err := a.PasswordResets.Create(token); err != nil {
				httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
		}
	}
	// The same shape and status are returned for unknown addresses. In a real
	// deployment this token would be delivered through an email provider.
	httpapi.WriteData(w, http.StatusOK, map[string]string{"reset_token": resetToken})
}

func (a *App) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request passwordResetConfirmBody
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	if request.ResetToken == "" {
		writeUnauthorized(w)
		return
	}
	if err := security.ValidatePassword(request.NewPassword); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid new password")
		return
	}
	reset, err := a.PasswordResets.Consume(security.HashOpaqueToken(request.ResetToken), a.Clock.Now().UTC())
	if err != nil {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.Get(reset.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	material, err := security.HashPassword(request.NewPassword)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid new password")
		return
	}
	user.PasswordMaterial = material
	if err := a.Users.Update(user); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Sessions.RevokeAllByUser(user.ID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"password_reset": true})
}
