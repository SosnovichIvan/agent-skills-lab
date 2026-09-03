package repository

import (
	"benchmark.local/iam/internal/domain"
	"sort"
	"sync"
)

type RoleRepository struct {
	mu    sync.RWMutex
	roles map[domain.RoleID]domain.Role
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{roles: make(map[domain.RoleID]domain.Role)}
}

func (r *RoleRepository) Create(role domain.Role) error {
	if role.ID == "" || role.OrganizationID == "" || role.Name == "" || role.BuiltIn || len(role.Permissions) == 0 {
		return domain.NewInvalid("invalid custom role")
	}
	seen := make(map[domain.Permission]struct{}, len(role.Permissions))
	for _, permission := range role.Permissions {
		if !domain.ValidPermission(permission) {
			return domain.NewInvalid("invalid permission")
		}
		if _, ok := seen[permission]; ok {
			return domain.NewInvalid("duplicate permission")
		}
		seen[permission] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.roles {
		if existing.OrganizationID == role.OrganizationID && existing.Name == role.Name {
			return domain.NewConflict("role already exists")
		}
	}
	if _, ok := r.roles[role.ID]; ok {
		return domain.NewConflict("role already exists")
	}
	role.Permissions = append([]domain.Permission(nil), role.Permissions...)
	r.roles[role.ID] = role
	return nil
}

func (r *RoleRepository) Get(organizationID domain.OrganizationID, id domain.RoleID) (domain.Role, error) {
	if builtin, ok := domain.BuiltInRole(string(id)); ok {
		builtin.OrganizationID = organizationID
		return builtin, nil
	}
	r.mu.RLock()
	role, ok := r.roles[id]
	r.mu.RUnlock()
	if !ok || role.OrganizationID != organizationID {
		return domain.Role{}, domain.NewNotFound("role not found")
	}
	role.Permissions = append([]domain.Permission(nil), role.Permissions...)
	return role, nil
}

func (r *RoleRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Role {
	result := make([]domain.Role, 0, 3)
	for _, name := range []string{"owner", "admin", "viewer"} {
		role, _ := r.Get(organizationID, domain.RoleID(name))
		result = append(result, role)
	}
	r.mu.RLock()
	for _, role := range r.roles {
		if role.OrganizationID == organizationID {
			role.Permissions = append([]domain.Permission(nil), role.Permissions...)
			result = append(result, role)
		}
	}
	r.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (r *RoleRepository) Delete(organizationID domain.OrganizationID, id domain.RoleID) error {
	if domain.IsBuiltInRole(string(id)) {
		return domain.NewConflict("built-in role cannot be deleted")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	role, ok := r.roles[id]
	if !ok || role.OrganizationID != organizationID {
		return domain.NewNotFound("role not found")
	}
	delete(r.roles, id)
	return nil
}
