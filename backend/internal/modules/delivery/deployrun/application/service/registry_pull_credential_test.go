package service

import (
	"testing"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestRegistryPullCredentialFromHarbor(t *testing.T) {
	cred := registryPullCredentialFromHarbor(deployport.DeploymentSecret{
		ID:          kernel.NewSecretID(),
		ServiceType: "Harbor",
		ServiceURL:  "http://192.168.5.24",
		Username:    "admin",
		Credentials: deployport.DeploymentCredentials{Password: "Harbor12345"},
	})
	if cred == nil {
		t.Fatal("expected credential")
	}
	if cred.Server != "192.168.5.24" {
		t.Fatalf("unexpected server: %q", cred.Server)
	}
	if cred.Username != "admin" || cred.Password != "Harbor12345" {
		t.Fatalf("unexpected credentials: %+v", cred)
	}
}

func TestRegistryPullCredentialFromHarborSkipsIncomplete(t *testing.T) {
	if registryPullCredentialFromHarbor(deployport.DeploymentSecret{
		ServiceType: "Harbor",
		ServiceURL:  "http://192.168.5.24",
	}) != nil {
		t.Fatal("expected nil for missing username/password")
	}
	if registryPullCredentialFromHarbor(deployport.DeploymentSecret{
		ServiceType: "Jenkins",
		ServiceURL:  "http://jenkins.local",
		Username:    "ci",
		Credentials: deployport.DeploymentCredentials{Password: "secret"},
	}) != nil {
		t.Fatal("expected nil for non-harbor secret")
	}
}
