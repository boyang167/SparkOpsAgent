package engine

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestParseBuildJobReadsRegistryAndProjectRows(t *testing.T) {
	spec := map[string]any{
		"registry_id": "sec-harbor",
		"project_builds": []any{
			map[string]any{"project_id": "p1", "branch": "release"},
		},
	}
	require.Equal(t, "sec-harbor", parseRegistryID(spec))
	require.Equal(t, []model.ProjectBuildSpec{{ProjectID: "p1", Branch: "release"}}, parseProjectBuilds(spec))
}

func TestImageRepositoryPathUsesRegistryHostAndProject(t *testing.T) {
	require.Equal(t, "192.168.5.24/team-a/orders", ImageRepositoryPath("https://192.168.5.24", "Team-A", "Orders"))
	require.Equal(t, "harbor.example/demo/app", ImageRepositoryPath("harbor.example", "demo", "app"))
	require.Empty(t, ImageRepositoryPath("", "demo", "app"))
}
