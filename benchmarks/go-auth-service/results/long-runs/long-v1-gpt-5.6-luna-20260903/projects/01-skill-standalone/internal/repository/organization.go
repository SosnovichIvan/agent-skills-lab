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

// Create stores an organization. OwnerID is mandatory because every
// organization must have an owner.
func (r *OrganizationRepository) Create(organization domain.Organization) error {
	if organization.ID == "" || organization.OwnerID == "" {
		return domain.NewInvalid("organization must have an owner")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[organization.ID]; exists {
		return domain.NewConflict("organization already exists")
	}
	r.organizations[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) GetByID(id domain.OrganizationID) (domain.Organization, error) {
	r.mu.RLock()
	organization, exists := r.organizations[id]
	r.mu.RUnlock()
	if !exists {
		return domain.Organization{}, domain.NewNotFound("organization not found")
	}
	return organization, nil
}

func (r *OrganizationRepository) Update(organization domain.Organization) error {
	if organization.ID == "" || organization.OwnerID == "" {
		return domain.NewInvalid("organization must have an owner")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[organization.ID]; !exists {
		return domain.NewNotFound("organization not found")
	}
	r.organizations[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) Delete(id domain.OrganizationID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.organizations[id]; !exists {
		return domain.NewNotFound("organization not found")
	}
	delete(r.organizations, id)
	return nil
}

func (r *OrganizationRepository) List() []domain.Organization {
	r.mu.RLock()
	result := make([]domain.Organization, 0, len(r.organizations))
	for _, organization := range r.organizations {
		result = append(result, organization)
	}
	r.mu.RUnlock()
	return result
}
