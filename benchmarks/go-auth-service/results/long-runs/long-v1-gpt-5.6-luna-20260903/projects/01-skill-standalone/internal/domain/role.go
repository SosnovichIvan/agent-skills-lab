package domain

import "time"

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

var allPermissions = []Permission{PermissionOrgRead, PermissionOrgManage, PermissionMemberRead, PermissionMemberManage, PermissionRoleRead, PermissionRoleManage, PermissionAPIKeyRead, PermissionAPIKeyManage, PermissionAuditRead}

type Role struct {
	ID             RoleID         `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	Name           string         `json:"name"`
	Permissions    []Permission   `json:"permissions"`
	BuiltIn        bool           `json:"built_in"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func BuiltInRole(name string) (Role, bool) {
	var permissions []Permission
	switch name {
	case "owner":
		permissions = append([]Permission(nil), allPermissions...)
	case "admin":
		permissions = append([]Permission(nil), allPermissions...)
	case "viewer":
		permissions = []Permission{PermissionOrgRead, PermissionMemberRead, PermissionRoleRead, PermissionAPIKeyRead, PermissionAuditRead}
	default:
		return Role{}, false
	}
	return Role{ID: RoleID(name), Name: name, Permissions: permissions, BuiltIn: true}, true
}

func IsBuiltInRole(name string) bool { _, ok := BuiltInRole(name); return ok }

func ValidPermission(permission Permission) bool {
	for _, allowed := range allPermissions {
		if permission == allowed {
			return true
		}
	}
	return false
}

func (r Role) HasPermission(permission Permission) bool {
	for _, granted := range r.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}
