package app

import (
	"context"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type claimsContextKey struct{}
type apiKeyContextKey struct{}

func ClaimsFromContext(ctx context.Context) (security.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(security.AccessTokenClaims)
	return claims, ok
}

func APIKeyFromContext(ctx context.Context) (domain.APIKey, bool) {
	key, ok := ctx.Value(apiKeyContextKey{}).(domain.APIKey)
	return key, ok
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || parts[1] == "" {
			writeUnauthorized(w, r)
			return
		}
		if strings.EqualFold(parts[0], "Bearer") {
			claims, err := a.Tokens.Validate(parts[1])
			if err != nil {
				writeUnauthorized(w, r)
				return
			}
			if !a.RateLimiter.allow("user:"+claims.Subject, a.Clock.Now().UTC(), a.Config.RateLimitRequests, a.Config.RateLimitWindow) {
				writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if strings.EqualFold(parts[0], "ApiKey") {
			key, err := a.APIKeys.FindByKeyHash(security.HashAPIKey(parts[1]))
			if err != nil || key.Revoked {
				writeUnauthorized(w, r)
				return
			}
			if !a.RateLimiter.allow("user:"+string(key.OwnerID), a.Clock.Now().UTC(), a.Config.RateLimitRequests, a.Config.RateLimitWindow) {
				writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			ctx := context.WithValue(r.Context(), apiKeyContextKey{}, key)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		writeUnauthorized(w, r)
		return
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
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
	writeData(w, http.StatusOK, user.PublicProfile())
}
