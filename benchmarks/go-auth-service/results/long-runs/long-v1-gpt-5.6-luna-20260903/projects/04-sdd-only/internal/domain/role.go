package domain

// Permission is an organization-scoped authorization capability.
type Permission string

const (
	OrgRead      Permission = "org.read"
	OrgManage    Permission = "org.manage"
	MemberRead   Permission = "member.read"
	MemberManage Permission = "member.manage"
	RoleRead     Permission = "role.read"
	RoleManage   Permission = "role.manage"
	APIKeyRead   Permission = "apikey.read"
	APIKeyManage Permission = "apikey.manage"
	AuditRead    Permission = "audit.read"
)

// Role is always owned by exactly one organization. BuiltIn roles use the
// fixed permission sets returned by BuiltInRole.
type Role struct {
	ID             ID           `json:"id"`
	OrganizationID ID           `json:"organization_id"`
	Name           string       `json:"name"`
	Permissions    []Permission `json:"permissions"`
	BuiltIn        bool         `json:"built_in"`
}

// BuiltInRole returns one of the fixed organization roles.
func BuiltInRole(organizationID ID, name string) (Role, bool) {
	permissions, ok := map[string][]Permission{
		OwnerRole:  {OrgRead, OrgManage, MemberRead, MemberManage, RoleRead, RoleManage, APIKeyRead, APIKeyManage, AuditRead},
		AdminRole:  {OrgRead, OrgManage, MemberRead, MemberManage, RoleRead, RoleManage, APIKeyRead, APIKeyManage, AuditRead},
		ViewerRole: {OrgRead, MemberRead},
	}[name]
	if !ok {
		return Role{}, false
	}
	return Role{ID: ID(string(organizationID) + ":" + name), OrganizationID: organizationID, Name: name, Permissions: append([]Permission(nil), permissions...), BuiltIn: true}, true
}
