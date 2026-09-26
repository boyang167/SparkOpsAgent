package model

import (
	spmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type (
	SprintStage                    = spmodel.SprintStage
	StageWorkflowBinding           = spmodel.StageWorkflowBinding
	StageGatePolicy                = spmodel.StageGatePolicy
	SprintProcess                  = spmodel.SprintProcess
	WorkspaceSprintProcessTemplate = spmodel.WorkspaceSprintProcessTemplate
)

func NormalizeSprintStages(stages []SprintStage) ([]SprintStage, error) {
	return spmodel.NormalizeSprintStages(stages)
}

func DefaultSprintStages() []SprintStage {
	return spmodel.DefaultSprintStages()
}

func NewSprintProcess(productID kernel.ProductID, name string, stages []SprintStage, createdBy kernel.MemberID) *SprintProcess {
	return spmodel.NewSprintProcess(productID, name, stages, createdBy)
}

func NewWorkspaceSprintProcessTemplate(
	workspaceID kernel.WorkspaceID,
	name, templateKey, description string,
	stages []SprintStage,
	createdBy kernel.MemberID,
) (*WorkspaceSprintProcessTemplate, error) {
	return spmodel.NewWorkspaceSprintProcessTemplate(workspaceID, name, templateKey, description, stages, createdBy)
}

func DefaultSprintProcessTemplates() map[string][]SprintStage {
	return spmodel.DefaultSprintProcessTemplates()
}
