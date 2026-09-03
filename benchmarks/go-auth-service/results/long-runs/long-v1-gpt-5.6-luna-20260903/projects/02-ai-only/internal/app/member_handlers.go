package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
)

type updateMemberRequest struct {
	Role string `json:"role"`
}

type transferOwnershipRequest struct {
	UserID domain.UserID `json:"user_id"`
}

func (a *App) updateMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	if _, err := a.Memberships.Get(orgID, domain.UserID(claims.Subject)); err != nil {
		writeUnauthorized(w, r)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	target, err := a.Memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	var request updateMemberRequest
	if err := decodeJSON(r, &request); err != nil || strings.TrimSpace(request.Role) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "member update is invalid")
		return
	}
	if target.Role == "owner" && request.Role != "owner" && a.Memberships.CountRole(orgID, "owner") <= 1 {
		writeError(w, r, http.StatusConflict, "conflict", "the last owner cannot lose owner role")
		return
	}
	target.Role = strings.TrimSpace(request.Role)
	target.UpdatedAt = a.Clock.Now().UTC()
	if err := a.Memberships.Update(target); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to update membership")
		return
	}
	a.audit(orgID, domain.UserID(claims.Subject), "member.updated", string(targetID))
	writeData(w, http.StatusOK, target)
}

func (a *App) deleteMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	if _, err := a.Memberships.Get(orgID, domain.UserID(claims.Subject)); err != nil {
		writeUnauthorized(w, r)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	target, err := a.Memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	if target.Role == "owner" && a.Memberships.CountRole(orgID, "owner") <= 1 {
		writeError(w, r, http.StatusConflict, "conflict", "the last owner cannot be removed")
		return
	}
	if err := a.Memberships.Delete(orgID, targetID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to delete membership")
		return
	}
	a.audit(orgID, domain.UserID(claims.Subject), "member.deleted", string(targetID))
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (a *App) transferOwnership(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	organization, err := a.Organizations.Get(orgID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	currentOwner := domain.UserID(claims.Subject)
	if organization.OwnerID != currentOwner {
		writeUnauthorized(w, r)
		return
	}
	var request transferOwnershipRequest
	if err := decodeJSON(r, &request); err != nil || request.UserID == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "new owner is invalid")
		return
	}
	a.OwnershipMu.Lock()
	defer a.OwnershipMu.Unlock()
	if err := a.Memberships.TransferOwnership(orgID, currentOwner, request.UserID); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict) {
			writeError(w, r, http.StatusConflict, "conflict", "ownership transfer is invalid")
			return
		}
		writeUnauthorized(w, r)
		return
	}
	if err := a.Organizations.TransferOwnership(orgID, currentOwner, request.UserID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to transfer ownership")
		return
	}
	organization.OwnerID = request.UserID
	a.audit(orgID, currentOwner, "ownership.transferred", string(request.UserID))
	writeData(w, http.StatusOK, organization)
}
