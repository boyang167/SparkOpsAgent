package service

import (
	"strings"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

func registryPullCredentialFromHarbor(harbor deployport.DeploymentSecret) *runtimeport.RegistryPullCredential {
	if !strings.EqualFold(strings.TrimSpace(harbor.ServiceType), "Harbor") {
		return nil
	}
	server := registryHostFromServiceURL(harbor.ServiceURL)
	username := strings.TrimSpace(harbor.Username)
	password := strings.TrimSpace(harbor.Credentials.Password)
	if server == "" || username == "" || password == "" {
		return nil
	}
	return &runtimeport.RegistryPullCredential{
		Server:   server,
		Username: username,
		Password: password,
	}
}

func registryHostFromServiceURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		if idx := strings.Index(trimmed, "://"); idx >= 0 {
			trimmed = trimmed[idx+3:]
		}
	}
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		trimmed = trimmed[:slash]
	}
	return strings.Trim(trimmed, "/")
}
