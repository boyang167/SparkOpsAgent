package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
)

type sysConfigOnlyRepo struct {
	repository.AdminRepository
	current *repository.SysConfig
	saved   *repository.SysConfig
}

func (r *sysConfigOnlyRepo) GetSysConfig(context.Context) (*repository.SysConfig, error) {
	return r.current, nil
}

func (r *sysConfigOnlyRepo) UpsertSysConfig(_ context.Context, cfg *repository.SysConfig) error {
	r.saved = cfg
	return nil
}

func TestUpdateSysConfigPreservesDedicatedSSOConfig(t *testing.T) {
	repo := &sysConfigOnlyRepo{
		current: &repository.SysConfig{
			Config: map[string]any{
				"features": map[string]any{"agent": true},
				"sso":      map[string]any{"provider": "ldap", "enabled": true},
			},
		},
	}
	handler := NewUpdateSysConfigHandler(repo)

	_, err := handler.Handle(
		context.Background(),
		"",
		"",
		false,
		map[string]any{
			"features": map[string]any{"agent": false},
			"sso":      map[string]any{"provider": "oidc", "enabled": false},
		},
	)

	require.NoError(t, err)
	require.Equal(t, map[string]any{"agent": false}, repo.saved.Config["features"])
	require.Equal(
		t,
		map[string]any{"provider": "ldap", "enabled": true},
		repo.saved.Config["sso"],
	)
}
