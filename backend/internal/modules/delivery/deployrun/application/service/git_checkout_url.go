package service

import (
	"strings"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	gitinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
)

// ResolveCheckoutGitURL picks the Git URL Jenkins should use for checkout.
// When the project stores an SSH clone URL but the secret is a token/password,
// rewrite to HTTPS via the GitLab service URL so Jenkins avoids SSH port issues.
func ResolveCheckoutGitURL(project deployport.ProjectDeploymentContext) string {
	gitURL := strings.TrimSpace(project.GitURL)
	if gitURL == "" {
		return gitURL
	}
	_, kind := CredentialValue(project.Git)
	if kind == "ssh-private-key" {
		return gitURL
	}
	if strings.HasPrefix(gitURL, "http://") || strings.HasPrefix(gitURL, "https://") {
		return gitURL
	}
	projectPath, err := gitinfra.ParseGitProjectPath(gitURL)
	if err != nil {
		return gitURL
	}
	base, err := gitinfra.ResolveGitLabAPIBase(gitURL, project.Git.ServiceURL)
	if err != nil {
		return gitURL
	}
	return strings.TrimSuffix(base, "/") + "/" + projectPath + ".git"
}
