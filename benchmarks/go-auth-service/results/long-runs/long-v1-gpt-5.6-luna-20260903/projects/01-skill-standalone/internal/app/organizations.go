package app

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"benchmark.local/iam/internal/domain"
)

type createOrganizationRequest struct {
	Name string `json:"name"`
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteResponse struct {
	Invite domain.Invite `json:"invite"`
	Token  string        `json:"token"`
}

type updateMemberRequest struct {
	Role string `json:"role"`
}

type transferOwnershipRequest struct {
	UserID     string `json:"user_id"`
	NewOwnerID string `json:"new_owner_id"`
	TargetID   string `json:"target_user_id"`
}

type createRoleRequest struct {
	Name        string              `json:"name"`
	Permissions []domain.Permission `json:"permissions"`
}

func (a *App) memberHasPermission(membership domain.Membership, permission domain.Permission) bool {
	role, err := a.roles.Get(membership.OrganizationID, domain.RoleID(membership.Role))
	return err == nil && role.HasPermission(permission)
}

func validBuiltInRole(role string) bool {
	return role == "owner" || role == "admin" || role == "viewer"
}

func (a *App) organizationMember(r *http.Request) (domain.AccessTokenClaims, domain.OrganizationID, domain.Membership, bool) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return claims, "", domain.Membership{}, false
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	membership, err := a.memberships.Get(orgID, domain.UserID(claims.Subject))
	if err != nil {
		return claims, orgID, domain.Membership{}, false
	}
	return claims, orgID, membership, true
}

func (a *App) listMembers(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, _, ok := a.organizationMember(r)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "organization membership required", requestID)
		return
	}
	members := a.memberships.ListByOrganization(orgID)
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	writeJSON(w, http.StatusOK, dataEnvelope{Data: members})
}

func (a *App) createInvite(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, orgID, membership, ok := a.organizationMember(r)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "organization membership required", requestID)
		return
	}
	if !a.memberHasPermission(membership, domain.PermissionMemberManage) {
		writeError(w, http.StatusForbidden, "forbidden", "member management permission required", requestID)
		return
	}
	idemKey, fingerprint, idemErr := idempotencyRequest(r)
	var idemEntry *idempotencyEntry
	idemStorageKey := "invite:" + string(orgID) + ":" + claims.Subject + ":" + idemKey
	if idemErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid idempotency request", requestID)
		return
	}
	if idemKey != "" {
		idemEntry, body, status, err := a.idempotency.begin(idemStorageKey, fingerprint, a.clock.Now().UTC())
		if err != nil {
			writeError(w, http.StatusConflict, "idempotency_conflict", err.Error(), requestID)
			return
		}
		if body != nil {
			replay(w, status, body)
			return
		}
		defer a.idempotency.fail(idemStorageKey, idemEntry)
	}
	var input inviteRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	email, err := domain.ValidateEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid email", requestID)
		return
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "viewer"
	}
	if role != "owner" && role != "admin" && role != "viewer" {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid role", requestID)
		return
	}
	token, err := domain.NewInviteToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	id, err := domain.NewInviteID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	invite := domain.Invite{ID: id, OrganizationID: orgID, Email: email, Role: role, TokenHash: domain.HashInviteToken(token), InvitedBy: domain.UserID(claims.Subject), CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}
	if err := a.invites.Create(invite); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.recordAudit(r, orgID, "invite.created", string(invite.ID), map[string]any{"email": invite.Email, "role": invite.Role})
	response := dataEnvelope{Data: inviteResponse{Invite: invite, Token: token}}
	if idemEntry != nil {
		a.idempotency.complete(idemStorageKey, idemEntry, http.StatusCreated, marshalResponse(response))
	}
	writeJSON(w, http.StatusCreated, response)
}

func (a *App) updateMember(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionMemberManage) {
		writeError(w, http.StatusForbidden, "forbidden", "member management permission required", requestID)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	current, err := a.memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found", requestID)
		return
	}
	if current.Role == "owner" && actor.Role != "owner" {
		writeError(w, http.StatusForbidden, "forbidden", "only the owner can change an owner", requestID)
		return
	}
	var input updateMemberRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	role := strings.TrimSpace(input.Role)
	if !validBuiltInRole(role) || (role == "owner" && actor.Role != "owner") {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid role", requestID)
		return
	}
	a.organizationMu.Lock()
	updated, err := a.memberships.UpdateRole(orgID, targetID, role, a.clock.Now().UTC())
	a.organizationMu.Unlock()
	if err != nil {
		status := http.StatusConflict
		code := "conflict"
		message := "member role cannot be changed"
		if domain.IsKind(err, domain.ErrNotFound) {
			status, code, message = http.StatusNotFound, "not_found", "member not found"
		}
		writeError(w, status, code, message, requestID)
		return
	}
	a.recordAudit(r, orgID, "member.role_changed", string(targetID), map[string]any{"role": updated.Role})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: updated})
}

func (a *App) deleteMember(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionMemberManage) {
		writeError(w, http.StatusForbidden, "forbidden", "member management permission required", requestID)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	target, err := a.memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found", requestID)
		return
	}
	if target.Role == "owner" {
		writeError(w, http.StatusConflict, "conflict", "ownership must be transferred first", requestID)
		return
	}
	a.organizationMu.Lock()
	err = a.memberships.DeleteMember(orgID, targetID)
	a.organizationMu.Unlock()
	if err != nil {
		status, code, message := http.StatusConflict, "conflict", "member cannot be deleted"
		if domain.IsKind(err, domain.ErrNotFound) {
			status, code, message = http.StatusNotFound, "not_found", "member not found"
		}
		writeError(w, status, code, message, requestID)
		return
	}
	a.recordAudit(r, orgID, "member.removed", string(targetID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) transferOwnership(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, orgID, actor, ok := a.organizationMember(r)
	if !ok || actor.Role != "owner" {
		writeError(w, http.StatusForbidden, "forbidden", "owner permission required", requestID)
		return
	}
	var input transferOwnershipRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	targetValue := strings.TrimSpace(input.UserID)
	if targetValue == "" {
		targetValue = strings.TrimSpace(input.NewOwnerID)
	}
	if targetValue == "" {
		targetValue = strings.TrimSpace(input.TargetID)
	}
	if targetValue == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "new owner is required", requestID)
		return
	}
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	now := a.clock.Now().UTC()
	organization, err := a.organizations.GetByID(orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found", requestID)
		return
	}
	_, newOwner, err := a.memberships.TransferOwnership(orgID, domain.UserID(claims.Subject), domain.UserID(targetValue), now)
	if err != nil {
		status, code, message := http.StatusConflict, "conflict", "ownership transfer failed"
		if domain.IsKind(err, domain.ErrNotFound) {
			status, code, message = http.StatusNotFound, "not_found", "member not found"
		}
		writeError(w, status, code, message, requestID)
		return
	}
	organization.OwnerID = newOwner.UserID
	organization.UpdatedAt = now
	if err := a.organizations.Update(organization); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.recordAudit(r, orgID, "organization.ownership_transferred", string(newOwner.UserID), nil)
	writeJSON(w, http.StatusOK, dataEnvelope{Data: organization})
}

func (a *App) acceptInvite(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" || !domain.ValidInviteToken(r.PathValue("token")) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid invite token", requestID)
		return
	}
	user, err := a.users.GetByID(domain.UserID(claims.Subject))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	now := a.clock.Now().UTC()
	tokenHash := domain.HashInviteToken(r.PathValue("token"))
	invite, err := a.invites.GetByToken(tokenHash)
	if err != nil || !now.Before(invite.ExpiresAt) || !strings.EqualFold(user.Email, invite.Email) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid invite token", requestID)
		return
	}
	invite, err = a.invites.Consume(tokenHash, now)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "invite already accepted", requestID)
		return
	}
	id, err := domain.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	membership := domain.Membership{ID: id, OrganizationID: invite.OrganizationID, UserID: user.ID, Role: invite.Role, CreatedAt: now, UpdatedAt: now}
	if err := a.memberships.Create(membership); err != nil {
		if domain.IsKind(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", "membership already exists", requestID)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.recordAudit(r, invite.OrganizationID, "invite.accepted", string(membership.ID), nil)
	writeJSON(w, http.StatusOK, dataEnvelope{Data: membership})
}

func (a *App) createOrganization(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	ownerID := domain.UserID(claims.Subject)
	if _, err := a.users.GetByID(ownerID); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}

	var input createOrganizationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "organization name is required", requestID)
		return
	}

	organizationID, err := domain.NewOrganizationID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	membershipID, err := domain.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	organization := domain.Organization{ID: organizationID, Name: name, OwnerID: ownerID, CreatedAt: now, UpdatedAt: now}
	if err := a.organizations.Create(organization); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	if err := a.memberships.Create(domain.Membership{ID: membershipID, OrganizationID: organizationID, UserID: ownerID, Role: "owner", CreatedAt: now, UpdatedAt: now}); err != nil {
		_ = a.organizations.Delete(organizationID)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.recordAudit(r, organizationID, "organization.created", string(organizationID), map[string]any{"name": organization.Name})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: organization})
}

func (a *App) listOrganizations(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}

	memberships := a.memberships.ListByUser(domain.UserID(claims.Subject))
	organizations := make([]domain.Organization, 0, len(memberships))
	for _, membership := range memberships {
		organization, err := a.organizations.GetByID(membership.OrganizationID)
		if err == nil {
			organizations = append(organizations, organization)
		}
	}
	sort.Slice(organizations, func(i, j int) bool { return organizations[i].ID < organizations[j].ID })
	writeJSON(w, http.StatusOK, dataEnvelope{Data: organizations})
}

func (a *App) createRole(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionRoleManage) {
		writeError(w, http.StatusForbidden, "forbidden", "role management permission required", requestID)
		return
	}
	var input createRoleRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || domain.IsBuiltInRole(name) || len(input.Permissions) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid role", requestID)
		return
	}
	id, err := domain.NewRoleID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	role := domain.Role{ID: id, OrganizationID: orgID, Name: name, Permissions: input.Permissions, CreatedAt: now, UpdatedAt: now}
	if err := a.roles.Create(role); err != nil {
		code, status := "invalid_request", http.StatusBadRequest
		if domain.IsKind(err, domain.ErrConflict) {
			code, status = "conflict", http.StatusConflict
		}
		writeError(w, status, code, err.Error(), requestID)
		return
	}
	a.recordAudit(r, orgID, "role.created", string(role.ID), map[string]any{"name": role.Name, "permissions": role.Permissions})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: role})
}

func (a *App) listRoles(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionRoleRead) {
		writeError(w, http.StatusForbidden, "forbidden", "role read permission required", requestID)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: a.roles.ListByOrganization(orgID)})
}

func (a *App) assignRole(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionRoleManage) {
		writeError(w, http.StatusForbidden, "forbidden", "role management permission required", requestID)
		return
	}
	roleID := domain.RoleID(r.PathValue("roleID"))
	role, err := a.roles.Get(orgID, roleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "role not found", requestID)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	_, err = a.memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found", requestID)
		return
	}
	if role.Name == "owner" && actor.Role != "owner" {
		writeError(w, http.StatusForbidden, "forbidden", "only the owner can assign owner", requestID)
		return
	}
	a.organizationMu.Lock()
	updated, err := a.memberships.UpdateRole(orgID, targetID, string(role.ID), a.clock.Now().UTC())
	a.organizationMu.Unlock()
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "role cannot be assigned", requestID)
		return
	}
	a.recordAudit(r, orgID, "member.role_assigned", string(targetID), map[string]any{"role_id": string(role.ID)})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: updated})
}

func (a *App) removeRole(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	_, orgID, actor, ok := a.organizationMember(r)
	if !ok || !a.memberHasPermission(actor, domain.PermissionRoleManage) {
		writeError(w, http.StatusForbidden, "forbidden", "role management permission required", requestID)
		return
	}
	role, err := a.roles.Get(orgID, domain.RoleID(r.PathValue("roleID")))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "role not found", requestID)
		return
	}
	targetID := domain.UserID(r.PathValue("userID"))
	target, err := a.memberships.Get(orgID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found", requestID)
		return
	}
	if target.Role != string(role.ID) {
		writeError(w, http.StatusNotFound, "not_found", "role assignment not found", requestID)
		return
	}
	a.organizationMu.Lock()
	updated, err := a.memberships.RemoveRole(orgID, targetID, string(role.ID), a.clock.Now().UTC())
	a.organizationMu.Unlock()
	if err != nil {
		status, code, message := http.StatusConflict, "conflict", "role cannot be removed"
		if domain.IsKind(err, domain.ErrNotFound) {
			status, code, message = http.StatusNotFound, "not_found", err.Error()
		}
		writeError(w, status, code, message, requestID)
		return
	}
	a.recordAudit(r, orgID, "member.role_removed", string(targetID), map[string]any{"role_id": string(role.ID)})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: updated})
}
