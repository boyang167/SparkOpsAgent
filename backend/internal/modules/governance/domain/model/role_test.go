package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestRoleHasPermission(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	role := model.NewRole(wsID, "editor", []string{"project:read", "project:write"}, nil)

	require.True(t, role.HasPermission("project:read"))
	require.True(t, role.HasPermission("project:write"))
	require.False(t, role.HasPermission("governance:admin"))
}

func TestDefaultWorkspaceRolesAreProtected(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	for _, name := range []string{"admin", "developer", "reviewer", "operator", "viewer"} {
		t.Run(name, func(t *testing.T) {
			require.True(t, model.NewRole(wsID, name, []string{"project:read"}, nil).IsProtected())
		})
	}
	require.False(t, model.NewRole(wsID, "custom", []string{"project:read"}, nil).IsProtected())
}
