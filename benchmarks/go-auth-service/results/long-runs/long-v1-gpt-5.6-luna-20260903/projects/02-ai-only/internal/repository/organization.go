package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type OrganizationRepository struct {
	mu   sync.RWMutex
	byID map[domain.OrganizationID]domain.Organization
}

func NewOrganizationRepository() *OrganizationRepository {
	return &OrganizationRepository{byID: make(map[domain.OrganizationID]domain.Organization)}
}

func (r *OrganizationRepository) Create(organization domain.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if organization.ID == "" || organization.OwnerID == "" || organization.Name == "" {
		return domain.Invalid("organization is incomplete")
	}
	if _, exists := r.byID[organization.ID]; exists {
		return domain.Conflict("organization already exists")
	}
	r.byID[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) Get(id domain.OrganizationID) (domain.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	organization, exists := r.byID[id]
	if !exists {
		return domain.Organization{}, domain.NotFound("organization not found")
	}
	return organization, nil
}

func (r *OrganizationRepository) Update(organization domain.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if organization.ID == "" || organization.OwnerID == "" || organization.Name == "" {
		return domain.Invalid("organization is incomplete")
	}
	if _, exists := r.byID[organization.ID]; !exists {
		return domain.NotFound("organization not found")
	}
	r.byID[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) TransferOwnership(id domain.OrganizationID, currentOwner, newOwner domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	organization, exists := r.byID[id]
	if !exists {
		return domain.NotFound("organization not found")
	}
	if organization.OwnerID != currentOwner {
		return domain.Unauthorized("current user is not the owner")
	}
	if newOwner == "" || newOwner == currentOwner {
		return domain.Invalid("new owner is invalid")
	}
	organization.OwnerID = newOwner
	organization.UpdatedAt = organization.UpdatedAt.UTC()
	r.byID[id] = organization
	return nil
}
