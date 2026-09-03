package app

import (
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
)

type organizationCreateRequest struct {
	Name string `json:"name"`
}

func (a *App) organizations(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	userID := domain.ID(claims.Subject)
	switch r.Method {
	case http.MethodPost:
		a.createOrganization(w, r, userID)
	case http.MethodGet:
		a.listOrganizations(w, userID)
	default:
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
	}
}

func (a *App) organizationSubresource(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/organizations/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		httpapi.WriteError(w, http.StatusNotFound, "not_found", "organization resource not found")
		return
	}
	orgID := domain.ID(parts[0])
	membership, err := a.Memberships.Get(orgID, domain.ID(claims.Subject))
	if err != nil {
		writeUnauthorized(w)
		return
	}
	switch {
	case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet:
		a.listMembers(w, orgID)
	case len(parts) == 2 && parts[1] == "invites" && r.Method == http.MethodPost:
		a.createInvite(w, r, orgID)
	case len(parts) == 2 && parts[1] == "ownership-transfer" && r.Method == http.MethodPost:
		a.transferOwnership(w, r, orgID, membership)
	case len(parts) == 2 && parts[1] == "api-keys" && r.Method == http.MethodPost:
		a.createAPIKey(w, r, orgID)
	case len(parts) == 2 && parts[1] == "api-keys" && r.Method == http.MethodGet:
		a.listAPIKeys(w, orgID)
	case len(parts) == 2 && parts[1] == "audit" && r.Method == http.MethodGet:
		a.listAudit(w, r, orgID)
	case len(parts) == 3 && parts[1] == "api-keys" && r.Method == http.MethodDelete:
		a.revokeAPIKey(w, r, orgID, domain.ID(parts[2]))
	case len(parts) == 3 && parts[1] == "members" && r.Method == http.MethodPatch:
		a.updateMember(w, r, orgID, domain.ID(parts[2]))
	case len(parts) == 3 && parts[1] == "members" && r.Method == http.MethodDelete:
		a.deleteMember(w, r, orgID, domain.ID(parts[2]))
	case len(parts) == 5 && parts[1] == "members" && parts[3] == "roles" && r.Method == http.MethodPut:
		a.assignRole(w, r, orgID, domain.ID(parts[2]), domain.ID(parts[4]))
	case len(parts) == 5 && parts[1] == "members" && parts[3] == "roles" && r.Method == http.MethodDelete:
		a.removeRole(w, r, orgID, domain.ID(parts[2]), domain.ID(parts[4]))
	default:
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
	}
}

func (a *App) createOrganization(w http.ResponseWriter, r *http.Request, ownerID domain.ID) {
	var request organizationCreateRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	if request.Name == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "organization name is required")
		return
	}
	organizationID, err := a.IDGenerator.NewID()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	now := a.Clock.Now().UTC()
	organization := domain.Organization{
		ID: organizationID, Name: request.Name, OwnerID: ownerID, CreatedAt: now,
	}
	if err := a.Organizations.Create(organization); err != nil {
		if errors.Is(err, domain.ErrInvalid) {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid organization")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Roles.EnsureBuiltInRoles(organizationID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Memberships.Create(domain.Membership{
		OrganizationID: organizationID,
		UserID:         ownerID,
		Role:           "owner",
		CreatedAt:      now,
	}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.appendAudit(organizationID, ownerID, "organization.created", nil); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusCreated, organization)
}

func (a *App) listOrganizations(w http.ResponseWriter, userID domain.ID) {
	memberships := a.Memberships.ListByUser(userID)
	organizations := make([]domain.Organization, 0, len(memberships))
	for _, membership := range memberships {
		organization, err := a.Organizations.Get(membership.OrganizationID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		organizations = append(organizations, organization)
	}
	httpapi.WriteData(w, http.StatusOK, organizations)
}
