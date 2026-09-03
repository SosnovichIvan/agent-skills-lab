package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type roleKey struct {
	organizationID domain.OrganizationID
	name           string
}

type RoleRepository struct {
	mu    sync.RWMutex
	byID  map[domain.RoleID]domain.Role
	byKey map[roleKey]domain.RoleID
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{byID: make(map[domain.RoleID]domain.Role), byKey: make(map[roleKey]domain.RoleID)}
}

func (r *RoleRepository) Create(role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if role.ID == "" || role.OrganizationID == "" || role.Name == "" {
		return domain.Invalid("role is incomplete")
	}
	if role.BuiltIn {
		fixed, ok := domain.BuiltInRole(role.Name)
		if !ok || !samePermissions(role.Permissions, fixed.Permissions) {
			return domain.Invalid("built-in role permissions are fixed")
		}
	} else if len(role.Permissions) == 0 {
		return domain.Invalid("custom role requires permissions")
	}
	if _, exists := r.byID[role.ID]; exists {
		return domain.Conflict("role already exists")
	}
	key := roleKey{organizationID: role.OrganizationID, name: role.Name}
	if _, exists := r.byKey[key]; exists {
		return domain.Conflict("role already exists in organization")
	}
	role.Permissions = clonePermissions(role.Permissions)
	r.byID[role.ID] = role
	r.byKey[key] = role.ID
	return nil
}

func (r *RoleRepository) Get(id domain.RoleID) (domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	role, exists := r.byID[id]
	if !exists {
		return domain.Role{}, domain.NotFound("role not found")
	}
	return cloneRole(role), nil
}

func (r *RoleRepository) GetByName(organizationID domain.OrganizationID, name string) (domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, exists := r.byKey[roleKey{organizationID: organizationID, name: name}]
	if !exists {
		return domain.Role{}, domain.NotFound("role not found")
	}
	return cloneRole(r.byID[id]), nil
}

func (r *RoleRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Role {
	r.mu.RLock()
	defer r.mu.RUnlock()
	roles := make([]domain.Role, 0)
	for _, role := range r.byID {
		if role.OrganizationID == organizationID {
			roles = append(roles, cloneRole(role))
		}
	}
	return roles
}

func samePermissions(left, right map[domain.Permission]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for permission, enabled := range right {
		if left[permission] != enabled {
			return false
		}
	}
	return true
}

func cloneRole(role domain.Role) domain.Role {
	role.Permissions = clonePermissions(role.Permissions)
	return role
}

func clonePermissions(permissions map[domain.Permission]bool) map[domain.Permission]bool {
	clone := make(map[domain.Permission]bool, len(permissions))
	for permission, enabled := range permissions {
		clone[permission] = enabled
	}
	return clone
}
