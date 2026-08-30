package policy

import (
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/require"
)

func TestMatcherAllowsPluginProxyPrefixForRolePolicies(t *testing.T) {
	m, err := model.NewModelFromString(modelConf)
	require.NoError(t, err)
	enforcer, err := casbin.NewEnforcer(m)
	require.NoError(t, err)

	require.True(t, mustAddPolicy(t, enforcer, "viewer", "ws-1", "/api/v1/plugins/kubernetes/clusters/*/proxy/*", "GET"))
	require.True(t, mustAddGroupingPolicy(t, enforcer, "member-1", "viewer", "ws-1"))

	allowed, err := enforcer.Enforce(
		"member-1",
		"ws-1",
		"/api/v1/plugins/kubernetes/clusters/abc/proxy/api/v1/pods",
		"GET",
	)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = enforcer.Enforce(
		"member-1",
		"ws-1",
		"/api/v1/plugins/kubernetes/clusters/abc/proxy/api/v1/pods",
		"POST",
	)
	require.NoError(t, err)
	require.False(t, allowed)
}

func mustAddPolicy(t *testing.T, enforcer *casbin.Enforcer, params ...any) bool {
	t.Helper()
	ok, err := enforcer.AddPolicy(params...)
	require.NoError(t, err)
	return ok
}

func mustAddGroupingPolicy(t *testing.T, enforcer *casbin.Enforcer, params ...any) bool {
	t.Helper()
	ok, err := enforcer.AddGroupingPolicy(params...)
	require.NoError(t, err)
	return ok
}
