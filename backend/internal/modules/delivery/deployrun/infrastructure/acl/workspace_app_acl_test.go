package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type aclFixture struct {
	db        *gorm.DB
	box       *crypto.SecretBox
	workspace kernel.WorkspaceID
	project   kernel.ProjectID
	cluster   kernel.ClusterInfoID
	other     kernel.ClusterInfoID
	secrets   map[string]kernel.SecretID
}

func newACLFixture(t *testing.T) *aclFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform_admin").Error)
	for _, ddl := range []string{
		`CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, namespace TEXT, git_url TEXT,
			secret_id TEXT, review_status TEXT, resources BLOB, default_cluster_id TEXT,
			jenkins_secret_id TEXT, kubernetes_secret_id TEXT)`,
		`CREATE TABLE workspace_app.secrets (
			id TEXT PRIMARY KEY, workspace_id TEXT, service_type TEXT, service_url TEXT,
			username TEXT, credential_blob TEXT, status TEXT)`,
		`CREATE TABLE platform_admin.cluster_info (
			id TEXT PRIMARY KEY, cluster_name TEXT, server TEXT, user_name TEXT,
			certificate_authority_data TEXT, client_certificate_data TEXT, client_key_data TEXT,
			bearer_token TEXT, is_build_env BOOLEAN, priority_id INTEGER)`,
		`CREATE TABLE platform_admin.workspace_clusters (
			workspace_id TEXT, cluster_id TEXT, created_by TEXT, created_at DATETIME,
			priority_id INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (workspace_id, cluster_id))`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	box, err := crypto.NewSecretBox(make([]byte, 32))
	require.NoError(t, err)
	f := &aclFixture{
		db: db, box: box, workspace: kernel.NewWorkspaceID(), project: kernel.NewProjectID(),
		cluster: kernel.NewClusterInfoID(), other: kernel.NewClusterInfoID(),
		secrets: map[string]kernel.SecretID{},
	}
	f.seed(t)
	return f
}

func (f *aclFixture) seed(t *testing.T) {
	t.Helper()
	actor := kernel.NewMemberID()
	require.NoError(t, f.db.Exec(
		`INSERT INTO platform_admin.cluster_info(id, cluster_name, server, bearer_token, is_build_env, priority_id)
		 VALUES (?, 'authorized', 'https://k8s.example', 'cluster-token', 0, 1), (?, 'other', 'https://other.example', 'other-token', 1, 2)`,
		f.cluster.String(), f.other.String()).Error)
	require.NoError(t, f.db.Exec(
		`INSERT INTO platform_admin.workspace_clusters(workspace_id, cluster_id, created_by, created_at, priority_id)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP, 0)`,
		f.workspace.String(), f.cluster.String(), actor.String()).Error)

	serviceTypes := []string{"GitLab", "Harbor", "Jenkins", "Kubernetes", "SonarQube"}
	for _, serviceType := range serviceTypes {
		id := kernel.NewSecretID()
		f.secrets[serviceType] = id
		creds := model.SecretCredentials{Token: serviceType + "-token"}
		if serviceType == "GitLab" {
			creds = model.SecretCredentials{
				Password: "password", Token: "token", APIKey: "api-key",
				PrivateKey: "private-key", PublicKey: "public-key",
				Certificate: "certificate", ExtraConfig: "git-extra",
			}
		}
		if serviceType == "Kubernetes" {
			creds.ExtraConfig = "apiVersion: v1\nclusters: []\n"
		}
		raw, err := json.Marshal(creds)
		require.NoError(t, err)
		encrypted, err := f.box.Encrypt(string(raw))
		require.NoError(t, err)
		require.NoError(t, f.db.Exec(
			`INSERT INTO workspace_app.secrets(id, workspace_id, service_type, service_url, username, credential_blob, status)
			 VALUES (?, ?, ?, ?, 'user', ?, 'active')`,
			id.String(), f.workspace.String(), serviceType, "https://"+serviceType+".example", encrypted).Error)
	}
	resources, err := json.Marshal(model.ProjectResources{
		RegistrySecretID:  f.secrets["Harbor"].String(),
		SonarqubeSecretID: f.secrets["SonarQube"].String(),
	})
	require.NoError(t, err)
	require.NoError(t, f.db.Exec(
		`INSERT INTO workspace_app.projects(
			id, workspace_id, name, namespace, git_url, secret_id, review_status, resources,
			default_cluster_id, jenkins_secret_id, kubernetes_secret_id)
		 VALUES (?, ?, 'demo', 'demo', 'https://git.example/demo.git', ?, 'approved', ?, ?, ?, ?)`,
		f.project.String(), f.workspace.String(), f.secrets["GitLab"].String(), resources,
		f.cluster.String(), f.secrets["Jenkins"].String(), f.secrets["Kubernetes"].String()).Error)
}

func (f *aclFixture) load() error {
	_, err := NewWorkspaceAppACL(f.db, f.box).Load(
		context.Background(), f.workspace, f.project, kernel.ClusterInfoID{})
	return err
}

func TestLoadRejectsUnauthorizedCluster(t *testing.T) {
	f := newACLFixture(t)
	_, err := NewWorkspaceAppACL(f.db, f.box).Load(
		context.Background(), f.workspace, f.project, f.other)
	require.ErrorIs(t, err, deploydomain.ErrDeploymentClusterNotAuthorized)
}

func TestLoadMapsDeploymentCredentialsWithoutWorkspaceModelLeak(t *testing.T) {
	f := newACLFixture(t)

	context, err := NewWorkspaceAppACL(f.db, f.box).Load(
		context.Background(), f.workspace, f.project, kernel.ClusterInfoID{})

	require.NoError(t, err)
	require.Equal(t, port.DeploymentCredentials{
		Password: "password", Token: "token", APIKey: "api-key",
		PrivateKey: "private-key", PublicKey: "public-key",
		Certificate: "certificate", ExtraConfig: "git-extra",
	}, context.Git.Credentials)
	require.Equal(t, "apiVersion: v1\nclusters: []\n", context.Kubernetes.Credentials.ExtraConfig)
	require.Equal(t, "harbor.example/demo/demo", context.ImageRepository)
	require.Contains(t, context.ServiceYAML, "kind: Service")
	require.Contains(t, context.DeploymentYAML, "kind: Deployment")
	require.NotContains(t, context.ServiceYAML+context.DeploymentYAML, "name: \"\"")
}

func TestLoadRejectsKubernetesSecretWithoutExtraConfig(t *testing.T) {
	f := newACLFixture(t)
	raw, _ := json.Marshal(model.SecretCredentials{Token: "only-token"})
	encrypted, err := f.box.Encrypt(string(raw))
	require.NoError(t, err)
	require.NoError(t, f.db.Exec(
		`UPDATE workspace_app.secrets SET credential_blob = ? WHERE id = ?`,
		encrypted, f.secrets["Kubernetes"].String()).Error)
	require.ErrorIs(t, f.load(), deploydomain.ErrKubernetesKubeconfigMissing)
}

func TestLoadMapsInvalidCredentialCipherAndJSONToSafeDomainError(t *testing.T) {
	t.Run("cipher", func(t *testing.T) {
		f := newACLFixture(t)
		require.NoError(t, f.db.Exec(
			`UPDATE workspace_app.secrets SET credential_blob = ? WHERE id = ?`,
			"must-not-leak-invalid-cipher", f.secrets["GitLab"].String()).Error)

		err := f.load()

		require.ErrorIs(t, err, deploydomain.ErrDeploymentBindingInvalid)
		require.NotContains(t, err.Error(), "must-not-leak")
	})
	t.Run("json", func(t *testing.T) {
		f := newACLFixture(t)
		encrypted, err := f.box.Encrypt("{not-json-secret")
		require.NoError(t, err)
		require.NoError(t, f.db.Exec(
			`UPDATE workspace_app.secrets SET credential_blob = ? WHERE id = ?`,
			encrypted, f.secrets["GitLab"].String()).Error)

		err = f.load()

		require.ErrorIs(t, err, deploydomain.ErrDeploymentBindingInvalid)
		require.NotContains(t, err.Error(), "not-json-secret")
	})
}

func TestLoadUsesClusterCredentialsWhenKubernetesSecretMissing(t *testing.T) {
	f := newACLFixture(t)
	require.NoError(t, f.db.Exec(
		`UPDATE workspace_app.projects SET kubernetes_secret_id = NULL WHERE id = ?`,
		f.project.String()).Error)

	context, err := NewWorkspaceAppACL(f.db, f.box).Load(
		context.Background(), f.workspace, f.project, kernel.ClusterInfoID{})

	require.NoError(t, err)
	require.Contains(t, context.Kubernetes.Credentials.ExtraConfig, "https://k8s.example")
	require.Contains(t, context.Kubernetes.Credentials.ExtraConfig, "cluster-token")
}

func TestLoadRejectsIncompleteBindings(t *testing.T) {
	cases := []struct {
		name, update string
		want         error
	}{
		{"unapproved", `review_status = 'none'`, deploydomain.ErrProjectNotApproved},
		{"missing git", `secret_id = NULL`, deploydomain.ErrDeploymentBindingMissing},
		{"missing harbor", `resources = '{}'`, deploydomain.ErrDeploymentBindingMissing},
		{"missing jenkins", `jenkins_secret_id = NULL`, deploydomain.ErrDeploymentBindingMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newACLFixture(t)
			require.NoError(t, f.db.Exec(
				fmt.Sprintf("UPDATE workspace_app.projects SET %s WHERE id = ?", tc.update),
				f.project.String()).Error)
			require.ErrorIs(t, f.load(), tc.want)
		})
	}
	t.Run("missing authorized cluster", func(t *testing.T) {
		f := newACLFixture(t)
		require.NoError(t, f.db.Exec(
			`DELETE FROM platform_admin.workspace_clusters WHERE workspace_id = ?`,
			f.workspace.String()).Error)
		require.ErrorIs(t, f.load(), deploydomain.ErrDeploymentClusterNotAuthorized)
	})
}

func TestLoadRequiresSonarOnlyWhenStaticScanEnabled(t *testing.T) {
	f := newACLFixture(t)
	resources, _ := json.Marshal(model.ProjectResources{RegistrySecretID: f.secrets["Harbor"].String()})
	require.NoError(t, f.db.Exec(`UPDATE workspace_app.projects SET resources = ? WHERE id = ?`, resources, f.project.String()).Error)
	require.NoError(t, f.load())

	resources, _ = json.Marshal(model.ProjectResources{
		RegistrySecretID: f.secrets["Harbor"].String(),
		EnableStaticScan: true,
	})
	require.NoError(t, f.db.Exec(`UPDATE workspace_app.projects SET resources = ? WHERE id = ?`, resources, f.project.String()).Error)
	require.ErrorIs(t, f.load(), deploydomain.ErrDeploymentBindingMissing)
}

func TestLoadRejectsInactiveOrWrongWorkspaceSecret(t *testing.T) {
	for _, tc := range []struct {
		name, update string
	}{
		{"inactive", `status = 'inactive'`},
		{"wrong workspace", `workspace_id = '00000000-0000-0000-0000-000000000000'`},
		{"wrong service", `service_type = 'Harbor'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newACLFixture(t)
			require.NoError(t, f.db.Exec(
				fmt.Sprintf("UPDATE workspace_app.secrets SET %s WHERE id = ?", tc.update),
				f.secrets["GitLab"].String()).Error)
			require.ErrorIs(t, f.load(), deploydomain.ErrDeploymentBindingMissing)
		})
	}
}

func TestListAuthorizedClustersOmitsUngranted(t *testing.T) {
	f := newACLFixture(t)
	items, err := NewWorkspaceAppACL(f.db, f.box).ListAuthorizedClusters(context.Background(), f.workspace)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, f.cluster, items[0].ID)
}
