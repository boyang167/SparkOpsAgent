package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8srbac "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/rbac"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestCreateListUpdateDeleteClusterUserGrant(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	adminID := kernel.NewMemberID()
	targetID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	seedIdentityMember(t, db, targetID.String(), "alice@example.com", "Alice")
	client := fake.NewSimpleClientset()
	prevIssuer := issueServiceAccountToken
	t.Cleanup(func() { issueServiceAccountToken = prevIssuer })
	issueServiceAccountToken = func(context.Context, kubernetes.Interface, string, string) (string, time.Time, error) {
		return "member-token", time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), nil
	}
	engine := testEngine(adminID, sparkplugin.PluginContext{
		DB: db,
		Clusters: fakeClusterFactory{
			items:  []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "dev"}},
			rest:   &rest.Config{Host: "https://k8s.example:6443", TLSClientConfig: rest.TLSClientConfig{Insecure: true}},
			client: client,
		},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{
			wsID: {adminID: true, targetID: true},
		}},
		Writer: allowWriter{allow: true},
	})

	create := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users", wsID, map[string]any{
		"member_ids":    []string{targetID.String()},
		"cluster_roles": []string{"view"},
		"namespace_roles": []map[string]any{{
			"namespace": "default",
			"roles":     []string{"edit"},
		}},
	})
	require.Equal(t, http.StatusOK, create.Code, create.Body.String())

	list := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users", wsID, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &env))
	body, err := json.Marshal(env.Data)
	require.NoError(t, err)
	require.Contains(t, string(body), `"email":"alice@example.com"`)
	require.Contains(t, string(body), `"display_name":"Alice"`)
	require.Contains(t, string(body), `"view"`)

	sa, err := client.CoreV1().ServiceAccounts(k8srbac.ManagedNamespace).Get(context.Background(), k8srbac.ServiceAccountName(targetID.String()), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "true", sa.Labels[k8srbac.LabelManaged])

	update := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users/"+targetID.String(), wsID, map[string]any{
		"cluster_roles":   []string{"cluster-admin"},
		"namespace_roles": []map[string]any{},
	})
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())

	kube := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users/"+targetID.String()+"/kubeconfig", wsID, nil)
	require.Equal(t, http.StatusOK, kube.Code, kube.Body.String())
	require.Contains(t, kube.Body.String(), "member-token")
	require.Contains(t, kube.Body.String(), "expires_at")

	del := performJSON(engine, http.MethodDelete, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users/"+targetID.String(), wsID, nil)
	require.Equal(t, http.StatusOK, del.Code, del.Body.String())
	_, err = client.CoreV1().ServiceAccounts(k8srbac.ManagedNamespace).Get(context.Background(), k8srbac.ServiceAccountName(targetID.String()), metav1.GetOptions{})
	require.Error(t, err)
}

func TestCreateClusterUserGrantRejectsNonMembersAndRequiresAdmin(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	adminID := kernel.NewMemberID()
	outsider := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(adminID, sparkplugin.PluginContext{
		DB: openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{
			items:  []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "dev"}},
			client: fake.NewSimpleClientset(),
		},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {adminID: true}}},
		Writer:  allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users", wsID, map[string]any{
		"member_ids":    []string{outsider.String()},
		"cluster_roles": []string{"view"},
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_USER_NOT_MEMBER")

	viewer := kernel.NewMemberID()
	forbidden := testEngine(viewer, sparkplugin.PluginContext{
		DB: openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{
			items:  []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "dev"}},
			client: fake.NewSimpleClientset(),
		},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {viewer: true}}},
		Writer:  allowWriter{allow: false},
	})
	denied := performJSON(forbidden, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/users", wsID, nil)
	require.Equal(t, http.StatusForbidden, denied.Code)
}

func seedIdentityMember(t *testing.T, db *gorm.DB, id, email, name string) {
	t.Helper()
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS identity").Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS identity.members (
		id TEXT PRIMARY KEY, email TEXT, display_name TEXT
	)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO identity.members (id, email, display_name) VALUES (?, ?, ?)`,
		id, email, name,
	).Error)
}
