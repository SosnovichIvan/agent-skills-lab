package app

import (
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
)

type createOrganizationRequest struct {
	Name string `json:"name"`
}

func (a *App) createOrganization(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	var request createOrganizationRequest
	if err := decodeJSON(r, &request); err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "organization name is required")
		return
	}
	organizationID, err := domain.NewOrganizationID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create organization")
		return
	}
	now := a.Clock.Now().UTC()
	organization := domain.Organization{
		ID: organizationID, Name: strings.TrimSpace(request.Name),
		OwnerID: domain.UserID(claims.Subject), CreatedAt: now, UpdatedAt: now,
	}
	if err := a.Organizations.Create(organization); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create organization")
		return
	}
	if err := a.Memberships.Create(domain.Membership{
		OrganizationID: organization.ID, UserID: organization.OwnerID,
		Role: "owner", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create organization membership")
		return
	}
	a.audit(organization.ID, organization.OwnerID, "organization.created", string(organization.ID))
	writeData(w, http.StatusCreated, organization)
}

func (a *App) listOrganizations(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w, r)
		return
	}
	memberships := a.Memberships.ListByUser(domain.UserID(claims.Subject))
	organizations := make([]domain.Organization, 0, len(memberships))
	for _, membership := range memberships {
		organization, err := a.Organizations.Get(membership.OrganizationID)
		if err == nil && organization.OwnerID != "" {
			organizations = append(organizations, organization)
		}
	}
	writeData(w, http.StatusOK, organizations)
}
