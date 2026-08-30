package query_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/stretchr/testify/require"
)

type queryRepository struct {
	plugins []*model.PluginRecord
}

func (r queryRepository) ListPlugins(context.Context) ([]*model.PluginRecord, error) {
	return r.plugins, nil
}

func (r queryRepository) FindPlugin(_ context.Context, id string) (*model.PluginRecord, error) {
	for _, plugin := range r.plugins {
		if plugin.ID == id {
			return plugin, nil
		}
	}
	return nil, domain.ErrPluginNotFound
}

func TestListAndGetPlugins(t *testing.T) {
	record := &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed}
	repo := queryRepository{plugins: []*model.PluginRecord{record}}

	items, err := query.NewListHandler(repo).Handle(context.Background())
	require.NoError(t, err)
	require.Equal(t, []*model.PluginRecord{record}, items)

	got, err := query.NewGetHandler(repo).Handle(context.Background(), "example")
	require.NoError(t, err)
	require.Equal(t, record, got)
}
