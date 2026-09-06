package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var jenkinsCredentialSegmentSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// StableCredentialID returns the legacy sparkops credential id format.
func StableCredentialID(workspaceID kernel.WorkspaceID, secretID kernel.SecretID) string {
	if secretID.UUID == uuid.Nil {
		return ""
	}
	return "sparkops-" + workspaceID.String() + "-" + secretID.String()
}

// JenkinsCredentialID builds a human-readable Jenkins credential id, e.g.
// sparkops-git-test-workspace-25049fe7-a8da-4da6-8a2a-67f93249145f
func JenkinsCredentialID(secret deployport.DeploymentSecret, namespace string) string {
	role := jenkinsCredentialRolePrefix(secret.ServiceType)
	ns := sanitizeJenkinsCredentialSegment(namespace)
	if ns == "" {
		ns = "default"
	}
	return fmt.Sprintf("sparkops-%s-%s-%s", role, ns, secret.ID.String())
}

func JenkinsCredentialDescription(secret deployport.DeploymentSecret, namespace string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "default"
	}
	desc := fmt.Sprintf("SparkOps 同步 · %s · 命名空间 %s", jenkinsCredentialRoleLabel(secret.ServiceType), ns)
	if serviceURL := strings.TrimSpace(secret.ServiceURL); serviceURL != "" {
		desc += fmt.Sprintf(" · %s", serviceURL)
	}
	return desc + fmt.Sprintf(" · secret %s", secret.ID.String())
}

func jenkinsCredentialRolePrefix(serviceType string) string {
	switch strings.TrimSpace(serviceType) {
	case "GitLab", "GitHub":
		return "git"
	case "Harbor":
		return "harbor"
	case "Jenkins":
		return "jenkins"
	case "SonarQube":
		return "sonar"
	default:
		return "secret"
	}
}

func jenkinsCredentialRoleLabel(serviceType string) string {
	switch strings.TrimSpace(serviceType) {
	case "GitLab":
		return "GitLab 代码仓库"
	case "GitHub":
		return "GitHub 代码仓库"
	case "Harbor":
		return "Harbor 镜像仓库"
	case "Jenkins":
		return "Jenkins 连接"
	case "SonarQube":
		return "SonarQube"
	default:
		return strings.TrimSpace(serviceType)
	}
}

func sanitizeJenkinsCredentialSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = jenkinsCredentialSegmentSanitizer.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func BuildJenkinsCredentials(_ kernel.WorkspaceID, project deployport.ProjectDeploymentContext) []deliveryport.JenkinsCredential {
	namespace := strings.TrimSpace(project.Namespace)
	secrets := []deployport.DeploymentSecret{project.Git, project.Harbor, project.Jenkins}
	if project.Sonar != nil {
		secrets = append(secrets, *project.Sonar)
	}
	credentials := make([]deliveryport.JenkinsCredential, 0, len(secrets))
	for _, secret := range secrets {
		if secret.ID.UUID == uuid.Nil {
			continue
		}
		value, kind := CredentialValue(secret)
		if strings.TrimSpace(value) == "" {
			continue
		}
		username := strings.TrimSpace(secret.Username)
		if kind == "username-password" && username == "" {
			username = defaultJenkinsCredentialUsername(secret.ServiceType)
		}
		if kind == "ssh-private-key" && username == "" {
			username = "git"
		}
		credentials = append(credentials, deliveryport.JenkinsCredential{
			ID:          JenkinsCredentialID(secret, namespace),
			Description: JenkinsCredentialDescription(secret, namespace),
			Username:    username,
			Secret:      value,
			Kind:        kind,
		})
	}
	return credentials
}

func defaultJenkinsCredentialUsername(serviceType string) string {
	switch strings.TrimSpace(serviceType) {
	case "Harbor":
		return "admin"
	case "Jenkins":
		return "admin"
	default:
		return "oauth2"
	}
}

func CredentialValue(secret deployport.DeploymentSecret) (string, string) {
	credentials := secret.Credentials
	if strings.TrimSpace(credentials.PrivateKey) != "" {
		return credentials.PrivateKey, "ssh-private-key"
	}
	values := []string{credentials.Password, credentials.Token, credentials.APIKey}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		return value, jenkinsCredentialKind(secret)
	}
	return "", ""
}

func jenkinsCredentialKind(secret deployport.DeploymentSecret) string {
	switch strings.TrimSpace(secret.ServiceType) {
	case "GitLab", "GitHub", "Harbor", "Jenkins":
		return "username-password"
	default:
		if strings.TrimSpace(secret.Username) != "" {
			return "username-password"
		}
		return "secret-text"
	}
}

func IsJenkinsManagedSecretType(serviceType string) bool {
	switch strings.TrimSpace(serviceType) {
	case "GitLab", "GitHub", "Harbor", "Jenkins", "SonarQube":
		return true
	default:
		return false
	}
}
