package authz

const (
	PermOrgRead            = "org:read"
	PermOrgMemberManage    = "org:member:manage"
	PermOrgWorkspaceCreate = "org:workspace:create"
	PermOrgFileAdmin       = "org:file:admin"
)

var orgRolePermissions = map[string][]string{
	"owner": {
		PermOrgRead,
		PermOrgMemberManage,
		PermOrgWorkspaceCreate,
		PermOrgFileAdmin,
	},
	"admin": {
		PermOrgRead,
		PermOrgMemberManage,
		PermOrgWorkspaceCreate,
		PermOrgFileAdmin,
	},
	"member": {
		PermOrgRead,
	},
}

func PermissionsForOrgRole(role string) []string {
	if perms, ok := orgRolePermissions[role]; ok {
		return append([]string(nil), perms...)
	}
	return nil
}

func OrgRoleHasPermission(role, permission string) bool {
	for _, p := range PermissionsForOrgRole(role) {
		if p == permission {
			return true
		}
	}
	return false
}
