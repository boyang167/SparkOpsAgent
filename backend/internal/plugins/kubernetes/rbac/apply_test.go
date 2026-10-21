package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestApplyCreatesManagedServiceAccountAndBindings(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	spec := Spec{
		WorkspaceID:  "ws-1",
		ClusterID:    "cl-1",
		MemberID:     "bc55a5d8-6ba0-47e2-887a-81ed7f57aa8e",
		ClusterRoles: []string{"view"},
		NamespaceRoles: []NamespaceRole{{
			Namespace: "default",
			Roles:     []string{"edit"},
		}},
	}

	require.NoError(t, Apply(ctx, client, spec))

	sa, err := client.CoreV1().ServiceAccounts(ManagedNamespace).Get(ctx, ServiceAccountName(spec.MemberID), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "true", sa.Labels[LabelManaged])
	require.Equal(t, spec.MemberID, sa.Labels[LabelMemberID])

	crbs, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, crbs.Items, 1)
	require.Equal(t, "view", crbs.Items[0].RoleRef.Name)
	require.Equal(t, "ServiceAccount", crbs.Items[0].Subjects[0].Kind)

	rbs, err := client.RbacV1().RoleBindings("default").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, rbs.Items, 1)
	require.Equal(t, "ClusterRole", rbs.Items[0].RoleRef.Kind)
	require.Equal(t, "edit", rbs.Items[0].RoleRef.Name)
}

func TestApplyReplacesStaleBindingsAndRemoveDeletesManagedObjects(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	memberID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	require.NoError(t, Apply(ctx, client, Spec{
		WorkspaceID:  "ws-1",
		ClusterID:    "cl-1",
		MemberID:     memberID,
		ClusterRoles: []string{"view", "edit"},
	}))
	require.NoError(t, Apply(ctx, client, Spec{
		WorkspaceID:  "ws-1",
		ClusterID:    "cl-1",
		MemberID:     memberID,
		ClusterRoles: []string{"view"},
	}))

	crbs, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, crbs.Items, 1)
	require.Equal(t, "view", crbs.Items[0].RoleRef.Name)

	require.NoError(t, Remove(ctx, client, "ws-1", "cl-1", memberID))
	crbs, err = client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, crbs.Items)
	_, err = client.CoreV1().ServiceAccounts(ManagedNamespace).Get(ctx, ServiceAccountName(memberID), metav1.GetOptions{})
	require.Error(t, err)
}
