package service

import (
	"strings"

	"github.com/google/uuid"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func StableCredentialID(workspaceID kernel.WorkspaceID, secretID kernel.SecretID) string {
	if secretID.UUID == uuid.Nil {
		return ""
	}
	return "sparkops-" + workspaceID.String() + "-" + secretID.String()
}

func BuildJenkinsCredentials(workspaceID kernel.WorkspaceID, project deployport.ProjectDeploymentContext) []deliveryport.JenkinsCredential {
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
		credentials = append(credentials, deliveryport.JenkinsCredential{
			ID:       StableCredentialID(workspaceID, secret.ID),
			Username: secret.Username,
			Secret:   value,
			Kind:     kind,
		})
	}
	return credentials
}

func CredentialValue(secret deployport.DeploymentSecret) (string, string) {
	credentials := secret.Credentials
	if strings.TrimSpace(credentials.PrivateKey) != "" {
		return credentials.PrivateKey, "ssh-private-key"
	}
	values := []string{credentials.Password, credentials.Token, credentials.APIKey}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			if strings.TrimSpace(secret.Username) != "" {
				return value, "username-password"
			}
			return value, "secret-text"
		}
	}
	return "", ""
}

func IsJenkinsManagedSecretType(serviceType string) bool {
	switch strings.TrimSpace(serviceType) {
	case "GitLab", "GitHub", "Harbor", "Jenkins", "SonarQube":
		return true
	default:
		return false
	}
}
