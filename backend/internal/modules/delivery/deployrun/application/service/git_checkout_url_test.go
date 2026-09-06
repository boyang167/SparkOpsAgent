package service

import (
	"testing"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestResolveCheckoutGitURLKeepsSSHWhenPrivateKeyConfigured(t *testing.T) {
	url := ResolveCheckoutGitURL(deployport.ProjectDeploymentContext{
		GitURL: "ssh://git@192.168.5.24:2224/cicd/test-repo.git",
		Git: deployport.DeploymentSecret{
			Credentials: deployport.DeploymentCredentials{PrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nabc"},
		},
	})
	require.Equal(t, "ssh://git@192.168.5.24:2224/cicd/test-repo.git", url)
}

func TestResolveCheckoutGitURLRewritesSSHToHTTPSForToken(t *testing.T) {
	url := ResolveCheckoutGitURL(deployport.ProjectDeploymentContext{
		GitURL: "ssh://git@192.168.5.24:2224/cicd/test-repo.git",
		Git: deployport.DeploymentSecret{
			ServiceURL:  "http://192.168.5.24:8929",
			Username:    "oauth2",
			Credentials: deployport.DeploymentCredentials{Token: "gitlab-token"},
		},
	})
	require.Equal(t, "http://192.168.5.24:8929/cicd/test-repo.git", url)
}

func TestResolveCheckoutGitURLKeepsHTTPS(t *testing.T) {
	url := ResolveCheckoutGitURL(deployport.ProjectDeploymentContext{
		GitURL: "http://192.168.5.24:8929/cicd/test-repo.git",
		Git: deployport.DeploymentSecret{
			Username:    "oauth2",
			Credentials: deployport.DeploymentCredentials{Token: "gitlab-token"},
		},
	})
	require.Equal(t, "http://192.168.5.24:8929/cicd/test-repo.git", url)
}

func TestResolveCheckoutGitURLIgnoresEmptyGitURL(t *testing.T) {
	require.Equal(t, "", ResolveCheckoutGitURL(deployport.ProjectDeploymentContext{
		ProjectID: kernel.NewProjectID(),
	}))

}
