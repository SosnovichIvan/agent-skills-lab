package app

import (
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

type memberUpdateRequest struct {
	Role string `json:"role"`
}

type ownershipTransferRequest struct {
	NewOwnerID domain.ID `json:"new_owner_id"`
}

func (a *App) updateMember(w http.ResponseWriter, r *http.Request, organizationID, userID domain.ID) {
	claims, _ := httpapi.ClaimsFromContext(r.Context())
	var request memberUpdateRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.Role == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid member update")
		return
	}
	membership, err := a.Memberships.Get(organizationID, userID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	membership.Role = request.Role
	if err := a.Memberships.Update(membership); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			httpapi.WriteError(w, http.StatusConflict, "conflict", "last organization owner cannot lose owner role")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "membership.updated", map[string]string{"user_id": string(userID), "role": request.Role}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, membership)
}

func (a *App) deleteMember(w http.ResponseWriter, r *http.Request, organizationID, userID domain.ID) {
	if err := a.Memberships.Delete(organizationID, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			httpapi.WriteError(w, http.StatusConflict, "conflict", "last organization owner cannot be removed")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if claims, ok := httpapi.ClaimsFromContext(r.Context()); ok {
		if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "membership.deleted", map[string]string{"user_id": string(userID)}); err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"removed": true})
}

func (a *App) transferOwnership(w http.ResponseWriter, r *http.Request, organizationID domain.ID, actor domain.Membership) {
	if actor.Role != domain.OwnerRole {
		writeUnauthorized(w)
		return
	}
	var request ownershipTransferRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.NewOwnerID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid ownership transfer")
		return
	}
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	if err := a.Memberships.TransferOwnership(organizationID, actor.UserID, request.NewOwnerID); err != nil {
		writeTransferError(w, err)
		return
	}
	if err := a.Organizations.TransferOwnership(organizationID, actor.UserID, request.NewOwnerID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.appendAudit(organizationID, actor.UserID, "ownership.transferred", map[string]string{"new_owner_id": string(request.NewOwnerID)}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"transferred": true})
}

func writeTransferError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "membership not found")
		return
	}
	if errors.Is(err, domain.ErrUnauthorized) {
		writeUnauthorized(w)
		return
	}
	httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
}
