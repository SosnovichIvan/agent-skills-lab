package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

// OrganizationRepository is a concurrency-safe in-memory organization store.
type OrganizationRepository struct {
	mu            sync.RWMutex
	organizations map[domain.OrganizationID]domain.Organization
}

func NewOrganizationRepository() *OrganizationRepository {
	return &OrganizationRepository{organizations: make(map[domain.OrganizationID]domain.Organization)}
}

// Create stores an organization. OwnerID is required because every
// organization must have an owner from the moment it is created.
func (r *OrganizationRepository) Create(organization domain.Organization) error {
	if r == nil || organization.ID == "" || organization.OwnerID == "" {
		return domain.NewError(domain.ErrorInvalid, "invalid organization")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[organization.ID]; exists {
		return domain.NewError(domain.ErrorConflict, "organization already exists")
	}
	r.organizations[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) GetByID(id domain.OrganizationID) (domain.Organization, error) {
	if r == nil {
		return domain.Organization{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	organization, ok := r.organizations[id]
	if !ok {
		return domain.Organization{}, domain.ErrNotFound
	}
	return organization, nil
}

// List returns a snapshot of all organizations.
func (r *OrganizationRepository) List() []domain.Organization {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	organizations := make([]domain.Organization, 0, len(r.organizations))
	for _, organization := range r.organizations {
		organizations = append(organizations, organization)
	}
	return organizations
}

func (r *OrganizationRepository) Update(organization domain.Organization) error {
	if r == nil || organization.ID == "" || organization.OwnerID == "" {
		return domain.NewError(domain.ErrorInvalid, "invalid organization")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[organization.ID]; !exists {
		return domain.ErrNotFound
	}
	r.organizations[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) Delete(id domain.OrganizationID) error {
	if r == nil {
		return domain.ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[id]; !exists {
		return domain.ErrNotFound
	}
	delete(r.organizations, id)
	return nil
}
