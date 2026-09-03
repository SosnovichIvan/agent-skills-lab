package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
)

func (a *App) assignRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	actorID := domain.UserID(claims.Subject)
	if _, err := a.Memberships.Get(orgID, actorID); err != nil {
		writeUnauthorized(w, r)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	if _, err := a.Memberships.Get(orgID, targetID); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	roleID := domain.RoleID(r.PathValue("roleID"))
	role, err := a.Roles.Get(roleID)
	if err != nil || role.OrganizationID != orgID {
		writeError(w, r, http.StatusNotFound, "not_found", "role not found")
		return
	}
	assignment := domain.RoleAssignment{OrganizationID: orgID, UserID: targetID, RoleID: roleID, AssignedAt: a.Clock.Now().UTC()}
	if err := a.RoleAssignments.Create(assignment); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, r, http.StatusConflict, "conflict", "role is already assigned")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "role assignment is invalid")
		return
	}
	a.audit(orgID, actorID, "role.assigned", string(roleID))
	writeData(w, http.StatusOK, assignment)
}

func (a *App) removeRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	actorID := domain.UserID(claims.Subject)
	if _, err := a.Memberships.Get(orgID, actorID); err != nil {
		writeUnauthorized(w, r)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	if _, err := a.Memberships.Get(orgID, targetID); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	roleID := domain.RoleID(r.PathValue("roleID"))
	role, err := a.Roles.Get(roleID)
	if err != nil || role.OrganizationID != orgID {
		writeError(w, r, http.StatusNotFound, "not_found", "role not found")
		return
	}
	if role.Name == "owner" && a.isLastOwner(orgID, targetID, roleID) {
		writeError(w, r, http.StatusConflict, "conflict", "the last owner cannot lose owner role")
		return
	}
	if err := a.RoleAssignments.Delete(orgID, targetID, roleID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "not_found", "role assignment not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to remove role")
		return
	}
	a.audit(orgID, actorID, "role.removed", string(roleID))
	writeData(w, http.StatusOK, map[string]bool{"removed": true})
}

func (a *App) isLastOwner(orgID domain.OrganizationID, userID domain.UserID, roleID domain.RoleID) bool {
	organization, err := a.Organizations.Get(orgID)
	if err == nil && organization.OwnerID == userID {
		return true
	}
	return a.RoleAssignments.Count(orgID, roleID) <= 1
}
