package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

func (a *App) emailVerificationRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.Get(domain.UserID(claims.Subject))
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	token, err := security.NewEmailVerificationToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create verification request")
		return
	}
	now := a.Clock.Now().UTC()
	verification, err := domain.NewID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create verification request")
		return
	}
	if err := a.Verifications.Create(domain.EmailVerification{
		ID: verification, UserID: user.ID,
		TokenHash: security.HashEmailVerificationToken(token),
		ExpiresAt: now.Add(a.Config.RefreshTokenTTL), CreatedAt: now,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create verification request")
		return
	}
	writeData(w, http.StatusAccepted, map[string]string{"verification_token": token})
}

func (a *App) emailVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &request); err != nil || request.Token == "" {
		writeUnauthorized(w, r)
		return
	}
	verification, consumeErr := a.Verifications.Consume(security.HashEmailVerificationToken(request.Token))
	if consumeErr != nil && !errors.Is(consumeErr, domain.ErrTokenReuse) {
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.Get(verification.UserID)
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	if errors.Is(consumeErr, domain.ErrTokenReuse) {
		if user.EmailVerification != domain.EmailVerified {
			writeUnauthorized(w, r)
			return
		}
		a.audit("", user.ID, "email.verification.confirmed", string(user.ID))
		writeData(w, http.StatusOK, user.PublicProfile())
		return
	}
	if !a.Clock.Now().UTC().Before(verification.ExpiresAt) {
		writeUnauthorized(w, r)
		return
	}
	user.EmailVerification = domain.EmailVerified
	user.UpdatedAt = a.Clock.Now().UTC()
	if err := a.Users.Update(user); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to verify email")
		return
	}
	a.audit("", user.ID, "email.verification.confirmed", string(user.ID))
	writeData(w, http.StatusOK, user.PublicProfile())
}
