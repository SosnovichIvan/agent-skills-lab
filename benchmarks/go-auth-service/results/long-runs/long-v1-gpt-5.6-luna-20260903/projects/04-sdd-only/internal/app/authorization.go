package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

// organizationPermissionMiddleware resolves the tenant from the URL and
// checks current repository state before an organization handler runs.
func (a *App) organizationPermissionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permission, ok := permissionForOrganizationRequest(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		orgID := organizationIDFromPath(r.URL.Path)
		if err := a.checkOrganizationPermission(r, orgID, permission); err != nil {
			writeAuthorizationError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) checkOrganizationPermission(r *http.Request, organizationID domain.ID, permission domain.Permission) error {
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok || organizationID == "" {
		return domain.ErrUnauthorized
	}
	membership, err := a.Memberships.Get(organizationID, domain.ID(claims.Subject))
	if err != nil {
		return domain.ErrUnauthorized
	}
	if builtIn, ok := domain.BuiltInRole(organizationID, membership.Role); ok && hasPermission(builtIn.Permissions, permission) {
		return nil
	}
	for _, roleID := range membership.RoleIDs {
		role, roleErr := a.Roles.Get(roleID)
		if roleErr != nil || role.OrganizationID != organizationID {
			continue
		}
		if hasPermission(role.Permissions, permission) {
			return nil
		}
	}
	return domain.NewError(domain.KindUnauthorized, "organization permission denied")
}

func permissionForOrganizationRequest(r *http.Request) (domain.Permission, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/organizations/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", false
	}
	if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet {
		return domain.MemberRead, true
	}
	if len(parts) == 2 && parts[1] == "invites" && r.Method == http.MethodPost {
		return domain.MemberManage, true
	}
	if len(parts) == 2 && parts[1] == "ownership-transfer" && r.Method == http.MethodPost {
		return domain.OrgManage, true
	}
	if len(parts) == 2 && parts[1] == "api-keys" && r.Method == http.MethodPost {
		return domain.APIKeyManage, true
	}
	if len(parts) == 2 && parts[1] == "api-keys" && r.Method == http.MethodGet {
		return domain.APIKeyRead, true
	}
	if len(parts) == 2 && parts[1] == "audit" && r.Method == http.MethodGet {
		return domain.AuditRead, true
	}
	if len(parts) == 3 && parts[1] == "api-keys" && r.Method == http.MethodDelete {
		return domain.APIKeyManage, true
	}
	if len(parts) == 3 && parts[1] == "members" && (r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
		return domain.MemberManage, true
	}
	if len(parts) == 5 && parts[1] == "members" && parts[3] == "roles" && (r.Method == http.MethodPut || r.Method == http.MethodDelete) {
		return domain.RoleManage, true
	}
	return "", false
}

func organizationIDFromPath(path string) domain.ID {
	path = strings.TrimPrefix(path, "/v1/organizations/")
	if index := strings.IndexByte(path, '/'); index >= 0 {
		path = path[:index]
	}
	return domain.ID(path)
}

func hasPermission(permissions []domain.Permission, wanted domain.Permission) bool {
	for _, permission := range permissions {
		if permission == wanted {
			return true
		}
	}
	return false
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrUnauthorized) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden", "permission denied")
		return
	}
	httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
}
