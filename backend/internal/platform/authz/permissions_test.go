package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemPermissionsByWorkspaceRole(t *testing.T) {
	tests := []struct {
		role string
		want map[string]bool
	}{
		{
			role: "admin",
			want: map[string]bool{
				PermFileRead: true, PermFileWrite: true, PermFileDelete: true,
				PermFileShare: true, PermFileAdmin: true, PermStorageRead: true,
				PermStorageAdmin: true, PermLifecycleAdmin: true,
			},
		},
		{
			role: "developer",
			want: map[string]bool{
				PermFileRead: true, PermFileWrite: true, PermFileDelete: true,
				PermFileShare: false, PermFileAdmin: false, PermStorageRead: false,
				PermStorageAdmin: false, PermLifecycleAdmin: false,
			},
		},
		{
			role: "operator",
			want: map[string]bool{
				PermFileRead: true, PermFileWrite: true, PermFileDelete: true,
				PermFileShare: false, PermFileAdmin: false, PermStorageRead: false,
				PermStorageAdmin: false, PermLifecycleAdmin: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			role := roleDefinition(t, tt.role)
			for permission, want := range tt.want {
				require.Equal(t, want, contains(role.Permissions, permission), permission)
			}
		})
	}
}

func TestOrganizationDefaultsDoNotGrantFileAdminBeforeGovernanceBootstrap(t *testing.T) {
	for _, role := range []string{"owner", "admin", "member"} {
		require.NotContains(t, PermissionsForOrgRole(role), PermOrgFileAdmin, role)
		require.False(t, OrgRoleHasPermission(role, PermOrgFileAdmin), role)
	}
}

func roleDefinition(t *testing.T, name string) DefaultRoleDefinition {
	t.Helper()
	for _, role := range DefaultWorkspaceRoles {
		if role.Name == name {
			return role
		}
	}
	t.Fatalf("role %q not found", name)
	return DefaultRoleDefinition{}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
