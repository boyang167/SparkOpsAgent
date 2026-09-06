package service

import (
	"testing"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestJenkinsCredentialIDUsesSemanticPrefix(t *testing.T) {
	gitID := kernel.NewSecretID()
	id := JenkinsCredentialID(deployport.DeploymentSecret{
		ID:          gitID,
		ServiceType: "GitLab",
	}, "test-workspace")
	require.Equal(t, "sparkops-git-test-workspace-"+gitID.String(), id)
}

func TestJenkinsCredentialDescriptionIncludesRoleAndNamespace(t *testing.T) {
	gitID := kernel.NewSecretID()
	desc := JenkinsCredentialDescription(deployport.DeploymentSecret{
		ID:          gitID,
		ServiceType: "GitLab",
		ServiceURL:  "http://192.168.5.24:8929",
	}, "test-workspace")
	require.Contains(t, desc, "SparkOps 同步")
	require.Contains(t, desc, "GitLab 代码仓库")
	require.Contains(t, desc, "test-workspace")
	require.Contains(t, desc, "8929")
	require.Contains(t, desc, gitID.String())
}

func TestBuildJenkinsCredentialsUsesUsernamePasswordForTokenOnlyGitLab(t *testing.T) {
	gitID := kernel.NewSecretID()
	credentials := BuildJenkinsCredentials(kernel.NewWorkspaceID(), deployport.ProjectDeploymentContext{
		Namespace: "test-workspace",
		Git: deployport.DeploymentSecret{
			ID:          gitID,
			ServiceType: "GitLab",
			ServiceURL:  "http://192.168.5.24:8929",
			Credentials: deployport.DeploymentCredentials{Token: "gitlab-pat"},
		},
	})
	require.Len(t, credentials, 1)
	require.Equal(t, "sparkops-git-test-workspace-"+gitID.String(), credentials[0].ID)
	require.Contains(t, credentials[0].Description, "GitLab 代码仓库")
	require.Equal(t, "username-password", credentials[0].Kind)
	require.Equal(t, "oauth2", credentials[0].Username)
	require.Equal(t, "gitlab-pat", credentials[0].Secret)
}

func TestBuildJenkinsCredentialsKeepsSonarAsSecretText(t *testing.T) {
	sonarID := kernel.NewSecretID()
	credentials := BuildJenkinsCredentials(kernel.NewWorkspaceID(), deployport.ProjectDeploymentContext{
		Namespace: "test-workspace",
		Sonar: &deployport.DeploymentSecret{
			ID:          sonarID,
			ServiceType: "SonarQube",
			Credentials: deployport.DeploymentCredentials{Token: "sonar-token"},
		},
	})
	require.Len(t, credentials, 1)
	require.Equal(t, "sparkops-sonar-test-workspace-"+sonarID.String(), credentials[0].ID)
	require.Equal(t, "secret-text", credentials[0].Kind)
}
