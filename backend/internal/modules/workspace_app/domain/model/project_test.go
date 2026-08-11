package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func mustName(t *testing.T, raw string) model.ProjectName {
	t.Helper()
	name, err := model.NewProjectName(raw)
	require.NoError(t, err)
	return name
}

func sampleInput(desc string) model.CreateProjectInput {
	return model.CreateProjectInput{Description: desc}
}

func TestProject_Archive(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput("desc"))
	require.NoError(t, p.Archive())
	require.Equal(t, model.ProjectStatusArchived, p.Status)
	require.ErrorIs(t, p.Archive(), domain.ErrProjectAlreadyArchived)
}

func TestProject_Rename_NotActive(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput(""))
	require.NoError(t, p.Archive())
	err := p.Rename(mustName(t, "new"))
	require.ErrorIs(t, err, domain.ErrProjectNotActive)
}

func TestProject_UpdateDetails(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput("old"))
	require.NoError(t, p.UpdateDetails(model.CreateProjectInput{Name: "new-name", Description: "new desc"}))
	require.Equal(t, "new-name", p.Name.String())
	require.Equal(t, "new desc", p.Description)
}

func TestNewProjectName_Invalid(t *testing.T) {
	_, err := model.NewProjectName("   ")
	require.ErrorIs(t, err, domain.ErrInvalidProjectName)
	_, err = model.NewProjectName("Bad_Name")
	require.ErrorIs(t, err, domain.ErrInvalidProjectName)
}
