package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

func (a *App) assignRole(w http.ResponseWriter, r *http.Request, organizationID, userID, roleID domain.ID) {
	role, err := a.Roles.Get(roleID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}
	if role.OrganizationID != organizationID {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}
	membership, err := a.Memberships.Get(organizationID, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	if role.BuiltIn {
		membership.Role = role.Name
	} else if !containsRoleID(membership.RoleIDs, role.ID) {
		membership.RoleIDs = append(membership.RoleIDs, role.ID)
	}
	if err := a.Memberships.Update(membership); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			httpapi.WriteError(w, http.StatusConflict, "conflict", "last organization owner cannot lose owner role")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if claims, ok := httpapi.ClaimsFromContext(r.Context()); ok {
		if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "role.assigned", map[string]string{"user_id": string(userID), "role_id": string(roleID)}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpapi.WriteData(w, http.StatusOK, membership)
}

func (a *App) removeRole(w http.ResponseWriter, r *http.Request, organizationID, userID, roleID domain.ID) {
	// 7.2:role revocation handler
	role, err := a.Roles.Get(roleID)
	if err != nil || role.OrganizationID != organizationID {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}
	membership, err := a.Memberships.Get(organizationID, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	if role.BuiltIn {
		if membership.Role != role.Name {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "role assignment not found")
			return
		}
		membership.Role = domain.ViewerRole
	} else {
		updated, found := removeRoleID(membership.RoleIDs, role.ID)
		if !found {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "role assignment not found")
			return
		}
		membership.RoleIDs = updated
	}
	if err := a.Memberships.Update(membership); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			httpapi.WriteError(w, http.StatusConflict, "conflict", "last organization owner cannot lose owner role")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if claims, ok := httpapi.ClaimsFromContext(r.Context()); ok {
		if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "role.revoked", map[string]string{"user_id": string(userID), "role_id": string(roleID)}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpapi.WriteData(w, http.StatusOK, membership)
}

func containsRoleID(ids []domain.ID, wanted domain.ID) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func removeRoleID(ids []domain.ID, wanted domain.ID) ([]domain.ID, bool) {
	for i, id := range ids {
		if id == wanted {
			return append(append([]domain.ID(nil), ids[:i]...), ids[i+1:]...), true
		}
	}
	return ids, false
}
