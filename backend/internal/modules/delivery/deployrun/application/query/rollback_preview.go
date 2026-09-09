package query

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RollbackDiffField struct {
	Label   string `json:"label"`
	Current string `json:"current"`
	Target  string `json:"target"`
	Changed bool   `json:"changed"`
}

type RollbackPreviewView struct {
	CanRollback bool                `json:"can_rollback"`
	Reason      string              `json:"reason,omitempty"`
	CurrentRun  *DeploymentView     `json:"current_run,omitempty"`
	TargetRun   *DeploymentView     `json:"target_run,omitempty"`
	Diff        []RollbackDiffField `json:"diff,omitempty"`
}

type RollbackPreviewHandler struct {
	repo repository.DeploymentRunRepository
}

func NewRollbackPreviewHandler(repo repository.DeploymentRunRepository) *RollbackPreviewHandler {
	return &RollbackPreviewHandler{repo: repo}
}

func (h *RollbackPreviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	targetRunID string,
) (RollbackPreviewView, error) {
	runs, err := h.repo.ListByProject(ctx, projectID, 50)
	if err != nil {
		return RollbackPreviewView{}, err
	}
	current, target, resolveErr := command.ResolveRollbackTarget(runs, wsID, projectID, strings.TrimSpace(targetRunID))
	if resolveErr != nil {
		reason := resolveErr.Error()
		if domainErr, ok := kernel.IsDomainError(resolveErr); ok {
			reason = domainErr.Error()
		}
		return RollbackPreviewView{CanRollback: false, Reason: reason}, nil
	}
	currentView := ToDeploymentView(current)
	targetView := ToDeploymentView(target)
	return RollbackPreviewView{
		CanRollback: true,
		CurrentRun:  &currentView,
		TargetRun:   &targetView,
		Diff:        buildRollbackDiff(current, target),
	}, nil
}

func buildRollbackDiff(current, target *model.DeploymentRun) []RollbackDiffField {
	fields := []struct {
		label   string
		current string
		target  string
	}{
		{"镜像 Tag", current.ImageTag, target.ImageTag},
		{"Git 引用", current.GitRefValue, target.GitRefValue},
		{"Git 类型", current.GitRefType, target.GitRefType},
		{"集群", current.ClusterID.String(), target.ClusterID.String()},
	}
	diff := make([]RollbackDiffField, 0, len(fields))
	for _, field := range fields {
		diff = append(diff, RollbackDiffField{
			Label:   field.label,
			Current: field.current,
			Target:  field.target,
			Changed: field.current != field.target,
		})
	}
	return diff
}
