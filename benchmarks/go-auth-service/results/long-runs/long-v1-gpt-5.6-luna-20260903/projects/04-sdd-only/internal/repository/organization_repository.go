package repository

import (
	"strings"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type OrganizationStore interface {
	Create(organization domain.Organization) error
	Get(id domain.ID) (domain.Organization, error)
	Update(organization domain.Organization) error
	Delete(id domain.ID) error
	TransferOwnership(id, currentOwnerID, newOwnerID domain.ID) error
}

type OrganizationRepository struct {
	mu   sync.RWMutex
	byID map[domain.ID]domain.Organization
}

func NewOrganizationRepository() *OrganizationRepository {
	return &OrganizationRepository{byID: make(map[domain.ID]domain.Organization)}
}

func (r *OrganizationRepository) Create(organization domain.Organization) error {
	if organization.ID == "" || organization.OwnerID == "" {
		return domain.NewError(domain.KindInvalid, "organization ID and owner are required")
	}
	if strings.TrimSpace(organization.Name) == "" {
		return domain.NewError(domain.KindInvalid, "organization name is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[organization.ID]; exists {
		return domain.NewError(domain.KindConflict, "organization already exists")
	}
	r.byID[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) Get(id domain.ID) (domain.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	organization, ok := r.byID[id]
	if !ok {
		return domain.Organization{}, domain.NewError(domain.KindNotFound, "organization not found")
	}
	return organization, nil
}

func (r *OrganizationRepository) Update(organization domain.Organization) error {
	if organization.ID == "" || organization.OwnerID == "" || strings.TrimSpace(organization.Name) == "" {
		return domain.NewError(domain.KindInvalid, "organization is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[organization.ID]; !exists {
		return domain.NewError(domain.KindNotFound, "organization not found")
	}
	r.byID[organization.ID] = organization
	return nil
}

func (r *OrganizationRepository) Delete(id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[id]; !exists {
		return domain.NewError(domain.KindNotFound, "organization not found")
	}
	delete(r.byID, id)
	return nil
}

func (r *OrganizationRepository) TransferOwnership(id, currentOwnerID, newOwnerID domain.ID) error {
	if currentOwnerID == "" || newOwnerID == "" || currentOwnerID == newOwnerID {
		return domain.NewError(domain.KindInvalid, "ownership transfer is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	organization, exists := r.byID[id]
	if !exists {
		return domain.NewError(domain.KindNotFound, "organization not found")
	}
	if organization.OwnerID != currentOwnerID {
		return domain.NewError(domain.KindUnauthorized, "current user is not owner")
	}
	organization.OwnerID = newOwnerID
	r.byID[id] = organization
	return nil
}
