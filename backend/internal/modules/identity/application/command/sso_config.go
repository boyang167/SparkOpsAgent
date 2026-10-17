package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
)

type SSOSecretUpdate struct {
	ClearClientSecret bool
	ClearBindPassword bool
}

type SSOConfigManager struct {
	repo    repository.SSOConfigRepository
	runtime *service.SSORuntime
}

func NewSSOConfigManager(
	repo repository.SSOConfigRepository,
	runtime *service.SSORuntime,
) *SSOConfigManager {
	return &SSOConfigManager{repo: repo, runtime: runtime}
}

func (m *SSOConfigManager) Get(ctx context.Context) (*model.SSOConfiguration, error) {
	cfg, err := m.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}
	snapshot := m.runtime.Snapshot()
	return &snapshot.Configuration, nil
}

func (m *SSOConfigManager) Test(
	ctx context.Context,
	provider model.SSOProvider,
	cfg model.SSOConfiguration,
	secrets SSOSecretUpdate,
) (service.ProviderTestResult, error) {
	current, err := m.Get(ctx)
	if err != nil {
		return service.ProviderTestResult{}, err
	}
	merged := mergeSSOSecrets(*current, cfg, secrets)
	return m.runtime.TestProvider(ctx, merged, provider)
}

func (m *SSOConfigManager) Save(
	ctx context.Context,
	expectedVersion int64,
	cfg model.SSOConfiguration,
	secrets SSOSecretUpdate,
) (*model.SSOConfiguration, error) {
	current, err := m.Get(ctx)
	if err != nil {
		return nil, err
	}
	merged := mergeSSOSecrets(*current, cfg, secrets)
	merged.Version = expectedVersion
	if err := m.runtime.Validate(merged); err != nil {
		return nil, err
	}
	saved, err := m.repo.Save(ctx, expectedVersion, merged)
	if err != nil {
		return nil, err
	}
	if err := m.runtime.Replace(*saved); err != nil {
		return nil, err
	}
	return saved, nil
}

func mergeSSOSecrets(
	current model.SSOConfiguration,
	next model.SSOConfiguration,
	update SSOSecretUpdate,
) model.SSOConfiguration {
	if update.ClearClientSecret {
		next.OIDC.ClientSecret = ""
	} else if next.OIDC.ClientSecret == "" {
		next.OIDC.ClientSecret = current.OIDC.ClientSecret
	}
	if update.ClearBindPassword {
		next.LDAP.BindPassword = ""
	} else if next.LDAP.BindPassword == "" {
		next.LDAP.BindPassword = current.LDAP.BindPassword
	}
	return next.Normalize()
}
