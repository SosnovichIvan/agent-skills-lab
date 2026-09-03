package domain

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

type Role struct {
	ID             RoleID              `json:"id"`
	OrganizationID OrganizationID      `json:"organization_id"`
	Name           string              `json:"name"`
	Permissions    map[Permission]bool `json:"permissions"`
	BuiltIn        bool                `json:"built_in"`
}

func BuiltInRole(name string) (Role, bool) {
	permissions, ok := builtInPermissions[name]
	if !ok {
		return Role{}, false
	}
	return Role{Name: name, BuiltIn: true, Permissions: clonePermissions(permissions)}, true
}

func BuiltInRolePermissions(name string) (map[Permission]bool, bool) {
	role, ok := BuiltInRole(name)
	if !ok {
		return nil, false
	}
	return role.Permissions, true
}

var builtInPermissions = map[string]map[Permission]bool{
	"owner": {
		PermissionOrgRead: true, PermissionOrgManage: true,
		PermissionMemberRead: true, PermissionMemberManage: true,
		PermissionRoleRead: true, PermissionRoleManage: true,
		PermissionAPIKeyRead: true, PermissionAPIKeyManage: true,
		PermissionAuditRead: true,
	},
	"admin": {
		PermissionOrgRead: true, PermissionOrgManage: true,
		PermissionMemberRead: true, PermissionMemberManage: true,
		PermissionRoleRead: true, PermissionRoleManage: true,
		PermissionAPIKeyRead: true, PermissionAPIKeyManage: true,
		PermissionAuditRead: true,
	},
	"viewer": {
		PermissionOrgRead: true, PermissionMemberRead: true,
		PermissionRoleRead: true, PermissionAPIKeyRead: true,
		PermissionAuditRead: true,
	},
}

func clonePermissions(permissions map[Permission]bool) map[Permission]bool {
	clone := make(map[Permission]bool, len(permissions))
	for permission, enabled := range permissions {
		clone[permission] = enabled
	}
	return clone
}
