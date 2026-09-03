package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
		return
	}
	user, err := a.Users.Get(domain.ID(claims.Subject))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, user.PublicProfile())
}
