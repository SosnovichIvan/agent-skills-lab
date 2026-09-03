package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type roleNameKey struct {
	organizationID domain.ID
	name           string
}

type RoleStore interface {
	EnsureBuiltInRoles(organizationID domain.ID) error
	Create(role domain.Role) error
	Get(id domain.ID) (domain.Role, error)
	GetByName(organizationID domain.ID, name string) (domain.Role, error)
	ListByOrganization(organizationID domain.ID) []domain.Role
	Update(role domain.Role) error
	Delete(id domain.ID) error
}

type RoleRepository struct {
	mu     sync.RWMutex
	byID   map[domain.ID]domain.Role
	byName map[roleNameKey]domain.ID
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{byID: make(map[domain.ID]domain.Role), byName: make(map[roleNameKey]domain.ID)}
}

// EnsureBuiltInRoles installs the fixed role set for one organization exactly
// once. It is safe to call when an organization is created more than once.
func (r *RoleRepository) EnsureBuiltInRoles(organizationID domain.ID) error {
	if organizationID == "" {
		return domain.NewError(domain.KindInvalid, "organization ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range []string{domain.OwnerRole, domain.AdminRole, domain.ViewerRole} {
		role, ok := domain.BuiltInRole(organizationID, name)
		if !ok {
			return domain.NewError(domain.KindInvalid, "built-in role is invalid")
		}
		if _, exists := r.byID[role.ID]; exists {
			continue
		}
		key := roleNameKey{organizationID: organizationID, name: name}
		if _, exists := r.byName[key]; exists {
			continue
		}
		r.byID[role.ID] = cloneRole(role)
		r.byName[key] = role.ID
	}
	return nil
}

func (r *RoleRepository) Create(role domain.Role) error {
	if err := validateRole(role); err != nil {
		return err
	}
	key := roleNameKey{organizationID: role.OrganizationID, name: role.Name}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[role.ID]; exists {
		return domain.NewError(domain.KindConflict, "role already exists")
	}
	if _, exists := r.byName[key]; exists {
		return domain.NewError(domain.KindConflict, "role name already exists")
	}
	r.byID[role.ID] = cloneRole(role)
	r.byName[key] = role.ID
	return nil
}

func (r *RoleRepository) Get(id domain.ID) (domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	role, ok := r.byID[id]
	if !ok {
		return domain.Role{}, domain.NewError(domain.KindNotFound, "role not found")
	}
	return cloneRole(role), nil
}

func (r *RoleRepository) GetByName(organizationID domain.ID, name string) (domain.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byName[roleNameKey{organizationID: organizationID, name: name}]
	if !ok {
		return domain.Role{}, domain.NewError(domain.KindNotFound, "role not found")
	}
	return cloneRole(r.byID[id]), nil
}

func (r *RoleRepository) ListByOrganization(organizationID domain.ID) []domain.Role {
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

func (r *RoleRepository) Update(role domain.Role) error {
	if err := validateRole(role); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.byID[role.ID]
	if !exists {
		return domain.NewError(domain.KindNotFound, "role not found")
	}
	if previous.OrganizationID != role.OrganizationID {
		return domain.NewError(domain.KindInvalid, "role organization cannot change")
	}
	newKey := roleNameKey{organizationID: role.OrganizationID, name: role.Name}
	if id, exists := r.byName[newKey]; exists && id != role.ID {
		return domain.NewError(domain.KindConflict, "role name already exists")
	}
	delete(r.byName, roleNameKey{organizationID: previous.OrganizationID, name: previous.Name})
	r.byID[role.ID] = cloneRole(role)
	r.byName[newKey] = role.ID
	return nil
}

func (r *RoleRepository) Delete(id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	role, exists := r.byID[id]
	if !exists {
		return domain.NewError(domain.KindNotFound, "role not found")
	}
	delete(r.byID, id)
	delete(r.byName, roleNameKey{organizationID: role.OrganizationID, name: role.Name})
	return nil
}

func validateRole(role domain.Role) error {
	if role.ID == "" || role.OrganizationID == "" || role.Name == "" {
		return domain.NewError(domain.KindInvalid, "role identifiers are required")
	}
	if role.BuiltIn {
		expected, ok := domain.BuiltInRole(role.OrganizationID, role.Name)
		if !ok || !samePermissions(role.Permissions, expected.Permissions) {
			return domain.NewError(domain.KindInvalid, "built-in role permissions are fixed")
		}
		return nil
	}
	if len(role.Permissions) == 0 {
		return domain.NewError(domain.KindInvalid, "custom role requires permissions")
	}
	for _, permission := range role.Permissions {
		if !knownPermission(permission) {
			return domain.NewError(domain.KindInvalid, "unknown role permission")
		}
	}
	return nil
}

func knownPermission(permission domain.Permission) bool {
	_, ok := map[domain.Permission]struct{}{
		domain.OrgRead: {}, domain.OrgManage: {}, domain.MemberRead: {}, domain.MemberManage: {},
		domain.RoleRead: {}, domain.RoleManage: {}, domain.APIKeyRead: {}, domain.APIKeyManage: {}, domain.AuditRead: {},
	}[permission]
	return ok
}

func samePermissions(left, right []domain.Permission) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[domain.Permission]int, len(left))
	for _, permission := range left {
		seen[permission]++
	}
	for _, permission := range right {
		seen[permission]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func cloneRole(role domain.Role) domain.Role {
	role.Permissions = append([]domain.Permission(nil), role.Permissions...)
	return role
}
