package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveMenuPolicyRoleIDs(t *testing.T) {
	roleIDsByName := map[string]string{
		"admin":     "role-admin",
		"developer": "role-dev",
		"viewer":    "role-viewer",
	}
	require.Equal(t, []string{"role-admin", "role-dev"}, resolveMenuPolicyRoleIDs(
		[]string{"admin", "developer", "admin"},
		roleIDsByName,
	))
	require.Equal(t, []string{"role-viewer"}, resolveMenuPolicyRoleIDs(
		[]string{"viewer", "missing"},
		roleIDsByName,
	))
	require.Empty(t, resolveMenuPolicyRoleIDs([]string{"qa"}, roleIDsByName))
}
