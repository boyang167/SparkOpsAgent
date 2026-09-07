package k8s

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

type MockManifestDeployer struct{}

func NewMockManifestDeployer() *MockManifestDeployer {
	return &MockManifestDeployer{}
}

func (d *MockManifestDeployer) Deploy(
	ctx context.Context,
	target port.DeploymentTarget,
	manifests []port.Manifest,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kubeconfig) == "" {
		return domain.ErrKubeconfigMissing
	}
	_, err := parseManifests(target, manifests)
	return err
}

func (d *MockManifestDeployer) RolloutRestart(ctx context.Context, target port.DeploymentTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kubeconfig) == "" {
		return domain.ErrKubeconfigMissing
	}
	return nil
}

var _ port.ManifestDeployer = (*MockManifestDeployer)(nil)
