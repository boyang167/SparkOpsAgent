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
