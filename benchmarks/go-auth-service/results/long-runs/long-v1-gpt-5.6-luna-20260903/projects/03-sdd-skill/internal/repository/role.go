package repository

import (
	"strings"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type roleKey struct {
	organizationID domain.OrganizationID
	name           string
}

// RoleRepository is a concurrency-safe organization-scoped role store.
type RoleRepository struct {
	mu        sync.RWMutex
	roles     map[domain.RoleID]domain.Role
	nameIndex map[roleKey]domain.RoleID
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{roles: make(map[domain.RoleID]domain.Role), nameIndex: make(map[roleKey]domain.RoleID)}
}

func cloneRole(role domain.Role) domain.Role {
	role.Permissions = append([]domain.Permission(nil), role.Permissions...)
	return role
}

func (r *RoleRepository) Create(role domain.Role) error {
	if r == nil {
		return domain.NewError(domain.ErrorInvalid, "invalid role")
	}
	if err := domain.ValidateRole(role); err != nil {
		return err
	}
	key := roleKey{organizationID: role.OrganizationID, name: role.Name}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.roles[role.ID]; ok {
		return domain.NewError(domain.ErrorConflict, "role already exists")
	}
	if _, ok := r.nameIndex[key]; ok {
		return domain.NewError(domain.ErrorConflict, "role already exists")
	}
	r.roles[role.ID] = cloneRole(role)
	r.nameIndex[key] = role.ID
	return nil
}

func (r *RoleRepository) GetByID(id domain.RoleID) (domain.Role, error) {
	if r == nil {
		return domain.Role{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	role, ok := r.roles[id]
	if !ok {
		return domain.Role{}, domain.ErrNotFound
	}
	return cloneRole(role), nil
}

func (r *RoleRepository) GetByOrganizationAndName(organizationID domain.OrganizationID, name string) (domain.Role, error) {
	if r == nil {
		return domain.Role{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.nameIndex[roleKey{organizationID: organizationID, name: strings.ToLower(strings.TrimSpace(name))}]
	if !ok {
		return domain.Role{}, domain.ErrNotFound
	}
	return cloneRole(r.roles[id]), nil
}

func (r *RoleRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Role {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Role, 0)
	for _, role := range r.roles {
		if role.OrganizationID == organizationID {
			result = append(result, cloneRole(role))
		}
	}
	return result
}
