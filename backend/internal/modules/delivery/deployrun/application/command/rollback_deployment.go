package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RollbackInput struct {
	TargetRunID string `json:"target_run_id,omitempty"`
}

type RollbackDeploymentHandler struct {
	repo  repository.DeploymentRunRepository
	queue messaging.Queue
	clock func() time.Time
}

func NewRollbackDeploymentHandler(
	repo repository.DeploymentRunRepository,
	queue messaging.Queue,
	clock func() time.Time,
) *RollbackDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	return &RollbackDeploymentHandler{repo: repo, queue: queue, clock: clock}
}

func (h *RollbackDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input RollbackInput,
) (*model.DeploymentRun, error) {
	active, err := h.repo.HasActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}

	runs, err := h.repo.ListByProject(ctx, projectID, 50)
	if err != nil {
		return nil, err
	}
	current, target, resolveErr := ResolveRollbackTarget(runs, wsID, projectID, strings.TrimSpace(input.TargetRunID))
	if resolveErr != nil {
		return nil, resolveErr
	}
	_ = current

	run := model.NewRollbackDeploymentRun(wsID, projectID, memberID, target)
	run.CreatedAt = h.clock().UTC()
	if err := h.repo.Create(ctx, run); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(consumer.Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})
	if err != nil {
		return nil, fmt.Errorf("marshal deployment message: %w", err)
	}
	if err := h.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
		run.Fail("publish deploy message: "+err.Error(), h.clock())
		if updateErr := h.repo.Update(ctx, run); updateErr != nil {
			return nil, errors.Join(err, fmt.Errorf("mark deployment failed: %w", updateErr))
		}
		return nil, err
	}
	return run, nil
}

func ResolveRollbackTarget(
	runs []*model.DeploymentRun,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	targetRunID string,
) (*model.DeploymentRun, *model.DeploymentRun, error) {
	succeeded := make([]*model.DeploymentRun, 0, len(runs))
	for _, run := range runs {
		if run.WorkspaceID != wsID || run.ProjectID != projectID {
			continue
		}
		if run.Status != model.StatusSucceeded || strings.TrimSpace(run.ImageTag) == "" {
			continue
		}
		succeeded = append(succeeded, run)
	}
	if len(succeeded) == 0 {
		return nil, nil, domain.ErrDeploymentNotRollbackable
	}

	current := succeeded[0]
	if targetRunID == "" {
		if len(succeeded) < 2 {
			return nil, nil, domain.ErrDeploymentNotRollbackable
		}
		return current, succeeded[1], nil
	}

	targetID, err := kernel.ParseDeploymentRunID(targetRunID)
	if err != nil {
		return nil, nil, domain.ErrDeploymentRunNotFound
	}
	if targetID == current.ID {
		return nil, nil, domain.ErrDeploymentNotRollbackable
	}
	for _, run := range succeeded {
		if run.ID == targetID {
			return current, run, nil
		}
	}
	return nil, nil, domain.ErrDeploymentRunNotFound
}
