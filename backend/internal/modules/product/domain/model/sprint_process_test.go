package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
)

func TestNormalizeSprintStages_assignsIDsAndSortOrder(t *testing.T) {
	stages, err := NormalizeSprintStages([]SprintStage{
		{Key: "dev", Name: "开发", SortOrder: 99},
		{Key: "test", Name: "测试"},
	})
	require.NoError(t, err)
	require.Len(t, stages, 2)
	require.NotEmpty(t, stages[0].ID)
	require.Equal(t, 0, stages[0].SortOrder)
	require.Equal(t, 1, stages[1].SortOrder)
}

func TestNormalizeSprintStages_rejectsDuplicateKeys(t *testing.T) {
	_, err := NormalizeSprintStages([]SprintStage{
		{ID: uuid.NewString(), Key: "dev", Name: "开发"},
		{ID: uuid.NewString(), Key: "dev", Name: "开发副本"},
	})
	require.ErrorIs(t, err, domain.ErrInvalidSprintProcess)
}

func TestNormalizeSprintStages_rejectsTooFewStages(t *testing.T) {
	_, err := NormalizeSprintStages([]SprintStage{
		{Key: "only", Name: "唯一"},
	})
	require.ErrorIs(t, err, domain.ErrInvalidSprintProcess)
}
