package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProject_ArchivingLifecycle(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput(""))

	require.NoError(t, p.BeginArchiving())
	require.True(t, p.IsArchiving())
	require.NotEmpty(t, p.ArchiveGeneration)
	require.Equal(t, model.ArchivePhasePending, p.ArchivePhase)
	firstGeneration := p.ArchiveGeneration
	require.ErrorIs(t, p.BeginArchiving(), domain.ErrProjectNotActive)
	require.ErrorIs(t, p.Rename(mustName(t, "other")), domain.ErrProjectNotActive)

	require.NoError(t, p.AbortArchiving())
	require.Equal(t, model.ProjectStatusActive, p.Status)
	require.Empty(t, p.ArchiveGeneration)
	require.Equal(t, model.ArchivePhaseNone, p.ArchivePhase)
	require.ErrorIs(t, p.AbortArchiving(), domain.ErrProjectNotArchiving)

	require.NoError(t, p.BeginArchiving())
	require.NotEqual(t, firstGeneration, p.ArchiveGeneration)
	require.NoError(t, p.Archive())
	require.Equal(t, model.ProjectStatusArchived, p.Status)
	require.Equal(t, model.ArchivePhaseNone, p.ArchivePhase)
	require.ErrorIs(t, p.Archive(), domain.ErrProjectAlreadyArchived)
}
