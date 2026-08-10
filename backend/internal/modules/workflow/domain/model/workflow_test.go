package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func mustWorkflowName(t *testing.T, raw string) model.WorkflowName {
	t.Helper()
	name, err := model.NewWorkflowName(raw)
	require.NoError(t, err)
	return name
}

func TestNewWorkflow_DefaultDefinition(t *testing.T) {
	wf := model.NewWorkflow(kernel.NewProjectID(), mustWorkflowName(t, "flow"), "desc", "")
	require.Equal(t, "{}", wf.Definition)
	require.Equal(t, model.WorkflowStatusDraft, wf.Status)
}

func TestWorkflow_Update_AfterArchived(t *testing.T) {
	wf := model.NewWorkflow(kernel.NewProjectID(), mustWorkflowName(t, "flow"), "", "{}")
	require.NoError(t, wf.Archive())
	err := wf.Update(mustWorkflowName(t, "new"), "x", "{}")
	require.ErrorIs(t, err, domain.ErrWorkflowArchived)
}

func TestWorkflow_Activate(t *testing.T) {
	wf := model.NewWorkflow(kernel.NewProjectID(), mustWorkflowName(t, "flow"), "", "{}")
	require.NoError(t, wf.Activate())
	require.Equal(t, model.WorkflowStatusActive, wf.Status)
}

func TestWorkflow_Archive(t *testing.T) {
	wf := model.NewWorkflow(kernel.NewProjectID(), mustWorkflowName(t, "flow"), "", "{}")
	require.NoError(t, wf.Archive())
	require.Equal(t, model.WorkflowStatusArchived, wf.Status)
}
