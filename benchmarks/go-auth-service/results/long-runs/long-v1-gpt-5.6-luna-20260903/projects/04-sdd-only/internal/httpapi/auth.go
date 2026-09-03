package httpapi

import (
	"context"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/security"
)

type claimsContextKey struct{}

// TokenValidator is the token validation dependency required by BearerMiddleware.
type TokenValidator interface {
	Validate(token string) (security.AccessTokenClaims, error)
}

// BearerMiddleware authenticates a request and stores validated claims in its
// context. It never forwards an invalid or missing bearer token.
func BearerMiddleware(validator TokenValidator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
			return
		}
		claims, err := validator.Validate(parts[1])
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
			return
		}
		requestContext := context.WithValue(r.Context(), claimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(requestContext))
	})
}

// ClaimsFromContext retrieves claims set by BearerMiddleware.
func ClaimsFromContext(ctx context.Context) (security.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(security.AccessTokenClaims)
	return claims, ok
}
