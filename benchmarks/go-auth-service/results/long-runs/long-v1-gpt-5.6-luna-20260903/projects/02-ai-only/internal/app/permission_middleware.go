package app

import (
	"net/http"

	"benchmark.local/iam/internal/domain"
)

func (a *App) requirePermission(permission domain.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		organizationID := domain.OrganizationID(r.PathValue("orgID"))
		if key, ok := APIKeyFromContext(r.Context()); ok {
			if key.OrganizationID != organizationID || !key.Scopes[permission] {
				writeUnauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeUnauthorized(w, r)
			return
		}
		if _, err := a.Organizations.Get(organizationID); err != nil || !a.hasPermission(organizationID, domain.UserID(claims.Subject), permission) {
			writeUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hasPermission reads memberships and role assignments on every request. No
// authorization data is copied into access tokens, so changes take effect
// immediately.
func (a *App) hasPermission(organizationID domain.OrganizationID, userID domain.UserID, permission domain.Permission) bool {
	membership, err := a.Memberships.Get(organizationID, userID)
	if err != nil {
		return false
	}
	if role, ok := domain.BuiltInRole(membership.Role); ok && role.Permissions[permission] {
		return true
	}
	for _, assignment := range a.RoleAssignments.ListByMember(organizationID, userID) {
		role, err := a.Roles.Get(assignment.RoleID)
		if err == nil && role.OrganizationID == organizationID && role.Permissions[permission] {
			return true
		}
	}
	return false
}
