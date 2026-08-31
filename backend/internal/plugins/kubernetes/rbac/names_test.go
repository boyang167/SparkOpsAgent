package rbac

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceAccountNameIsDNS1123AndStable(t *testing.T) {
	memberID := "bc55a5d8-6ba0-47e2-887a-81ed7f57aa8e"
	name := ServiceAccountName(memberID)
	require.Equal(t, ServiceAccountName(memberID), name)
	require.True(t, strings.HasPrefix(name, "sparkops-"))
	require.LessOrEqual(t, len(name), 63)
	require.NotContains(t, name, "_")
	require.NotEqual(t, ServiceAccountName("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), name)
}

func TestBindingNamesStayShortAndDistinct(t *testing.T) {
	memberID := "23bfff53-6cd3-4734-997e-05e10211fff3"
	crb := ClusterRoleBindingName(memberID, "cluster-admin")
	rb := RoleBindingName(memberID, "kube-system", "view")
	require.LessOrEqual(t, len(crb), 63)
	require.LessOrEqual(t, len(rb), 63)
	require.NotEqual(t, crb, ClusterRoleBindingName(memberID, "view"))
	require.NotEqual(t, rb, RoleBindingName(memberID, "default", "view"))
}
