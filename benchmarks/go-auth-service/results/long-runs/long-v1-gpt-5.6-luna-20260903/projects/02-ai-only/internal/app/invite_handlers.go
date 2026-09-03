package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

type createInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (a *App) createInvite(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	orgID := domain.OrganizationID(r.PathValue("orgID"))
	if _, err := a.Organizations.Get(orgID); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if _, err := a.Memberships.Get(orgID, domain.UserID(claims.Subject)); err != nil {
		writeUnauthorized(w, r)
		return
	}
	var request createInviteRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	email, err := domain.NormalizeEmail(request.Email)
	if err != nil || strings.TrimSpace(request.Role) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "invite is invalid")
		return
	}
	token, err := security.NewInviteToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create invite")
		return
	}
	now := a.Clock.Now().UTC()
	id, err := domain.NewInviteID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create invite")
		return
	}
	if err := a.Invites.Create(domain.Invite{ID: id, OrganizationID: orgID, InvitedEmail: domain.Email(email), Role: strings.TrimSpace(request.Role), TokenHash: security.HashInviteToken(token), ExpiresAt: now.Add(a.Config.RefreshTokenTTL), CreatedBy: domain.UserID(claims.Subject), CreatedAt: now}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create invite")
		return
	}
	a.audit(orgID, domain.UserID(claims.Subject), "invite.created", string(id))
	writeData(w, http.StatusCreated, map[string]string{"invite_token": token})
}

func (a *App) acceptInvite(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	invite, err := a.Invites.Consume(security.HashInviteToken(r.PathValue("token")))
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.Get(domain.UserID(claims.Subject))
	if err != nil || domain.Email(strings.ToLower(strings.TrimSpace(string(user.Email)))) != invite.InvitedEmail || !a.Clock.Now().UTC().Before(invite.ExpiresAt) {
		writeUnauthorized(w, r)
		return
	}
	now := a.Clock.Now().UTC()
	if err := a.Memberships.Create(domain.Membership{OrganizationID: invite.OrganizationID, UserID: user.ID, Role: invite.Role, CreatedAt: now, UpdatedAt: now}); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, r, http.StatusConflict, "conflict", "membership already exists")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to accept invite")
		return
	}
	a.audit(invite.OrganizationID, user.ID, "invite.accepted", string(user.ID))
	writeData(w, http.StatusOK, map[string]any{"organization_id": invite.OrganizationID, "role": invite.Role})
}

func (a *App) listMembers(w http.ResponseWriter, r *http.Request) {
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
	memberships := a.Memberships.ListByOrganization(orgID)
	profiles := make([]domain.PublicProfile, 0, len(memberships))
	for _, membership := range memberships {
		if user, err := a.Users.Get(membership.UserID); err == nil {
			profiles = append(profiles, user.PublicProfile())
		}
	}
	writeData(w, http.StatusOK, profiles)
}
