package domain

import (
	"sort"
	"strings"
	"time"
)

// Permission is an authorization capability scoped to an organization.
type Permission string

const (
	PermissionOrgRead      Permission = "org.read"
	PermissionOrgManage    Permission = "org.manage"
	PermissionMemberRead   Permission = "member.read"
	PermissionMemberManage Permission = "member.manage"
	PermissionRoleRead     Permission = "role.read"
	PermissionRoleManage   Permission = "role.manage"
	PermissionAPIKeyRead   Permission = "apikey.read"
	PermissionAPIKeyManage Permission = "apikey.manage"
	PermissionAuditRead    Permission = "audit.read"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

var builtInPermissions = map[string][]Permission{
	RoleOwner: {
		PermissionOrgRead, PermissionOrgManage, PermissionMemberRead,
		PermissionMemberManage, PermissionRoleRead, PermissionRoleManage,
		PermissionAPIKeyRead, PermissionAPIKeyManage, PermissionAuditRead,
	},
	RoleAdmin: {
		PermissionOrgRead, PermissionOrgManage, PermissionMemberRead,
		PermissionMemberManage, PermissionRoleRead, PermissionRoleManage,
		PermissionAPIKeyRead, PermissionAPIKeyManage, PermissionAuditRead,
	},
	RoleViewer: {PermissionOrgRead, PermissionMemberRead, PermissionRoleRead},
}

// BuiltInRolePermissions returns a copy of the fixed permission set for name.
func BuiltInRolePermissions(name string) ([]Permission, bool) {
	permissions, ok := builtInPermissions[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, false
	}
	return append([]Permission(nil), permissions...), true
}

// Role describes either one of the built-in roles or an organization custom role.
type Role struct {
	ID             RoleID         `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	Name           string         `json:"name"`
	Permissions    []Permission   `json:"permissions"`
	BuiltIn        bool           `json:"built_in"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// NewBuiltInRole creates a role with the canonical permissions for name.
func NewBuiltInRole(id RoleID, organizationID OrganizationID, name string, now time.Time) (Role, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	permissions, ok := BuiltInRolePermissions(name)
	if !ok || id == "" || organizationID == "" {
		return Role{}, NewError(ErrorInvalid, "invalid built-in role")
	}
	return Role{ID: id, OrganizationID: organizationID, Name: name, Permissions: permissions, BuiltIn: true, CreatedAt: now, UpdatedAt: now}, nil
}

// ValidateRole checks organization ownership, custom permission presence, and
// that built-in roles cannot be modified into a different permission set.
func ValidateRole(role Role) error {
	if role.ID == "" || role.OrganizationID == "" || strings.TrimSpace(role.Name) == "" {
		return NewError(ErrorInvalid, "invalid role")
	}
	role.Name = strings.ToLower(strings.TrimSpace(role.Name))
	want, builtIn := BuiltInRolePermissions(role.Name)
	if role.BuiltIn != builtIn {
		return NewError(ErrorInvalid, "invalid role")
	}
	if len(role.Permissions) == 0 {
		return NewError(ErrorInvalid, "role permissions are required")
	}
	if builtIn && !samePermissions(role.Permissions, want) {
		return NewError(ErrorInvalid, "invalid built-in role permissions")
	}
	return nil
}

func samePermissions(left, right []Permission) bool {
	if len(left) != len(right) {
		return false
	}
	l := append([]Permission(nil), left...)
	r := append([]Permission(nil), right...)
	sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	for i := range l {
		if l[i] != r[i] {
			return false
		}
	}
	return true
}
