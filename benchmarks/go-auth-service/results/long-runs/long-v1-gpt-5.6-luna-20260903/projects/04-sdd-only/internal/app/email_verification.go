package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/security"
)

type emailVerificationConfirmRequest struct {
	VerificationToken string `json:"verification_token"`
}

func (a *App) emailVerificationRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.Get(domain.ID(claims.Subject))
	if err != nil {
		writeUnauthorized(w)
		return
	}
	tokenValue, err := security.NewEmailVerificationToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	tokenID, err := a.IDGenerator.NewID()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	token := domain.EmailVerificationToken{
		ID: tokenID, UserID: user.ID,
		TokenHash: security.HashOpaqueToken(tokenValue),
		ExpiresAt: a.Clock.Now().UTC().Add(a.Config.EmailVerificationTTL),
	}
	if err := a.EmailVerifications.Create(token); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]string{"verification_token": tokenValue})
}

func (a *App) emailVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request emailVerificationConfirmRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.VerificationToken == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	token, err := a.EmailVerifications.Consume(security.HashOpaqueToken(request.VerificationToken), a.Clock.Now().UTC())
	if err != nil {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.Get(token.UserID)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if user.EmailVerification == domain.VerificationVerified {
		httpapi.WriteData(w, http.StatusOK, map[string]bool{"email_verified": true})
		return
	}
	user.EmailVerification = domain.VerificationVerified
	if err := a.Users.Update(user); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeUnauthorized(w)
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"email_verified": true})
}
