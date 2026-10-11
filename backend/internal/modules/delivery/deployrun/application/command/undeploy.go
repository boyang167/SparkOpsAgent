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
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	projectStatusActive    = "active"
	projectStatusArchiving = "archiving"
	undeployBaselineWindow = 100
)

type UndeployInput struct {
	ClusterID     string `json:"cluster_id"`
	ConfirmName   string `json:"confirm_name"`
	DeleteConfig  bool   `json:"delete_config"`
	DeleteStorage bool   `json:"delete_storage"`
}

type UndeployHandler struct {
	repo      repository.DeploymentRunRepository
	lifecycle port.ProjectLifecycleReader
	queue     messaging.Queue
	clock     func() time.Time
}

func NewUndeployHandler(
	repo repository.DeploymentRunRepository,
	lifecycle port.ProjectLifecycleReader,
	queue messaging.Queue,
	clock func() time.Time,
) *UndeployHandler {
	if clock == nil {
		clock = time.Now
	}
	return &UndeployHandler{repo: repo, lifecycle: lifecycle, queue: queue, clock: clock}
}

func (h *UndeployHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input UndeployInput,
) (*model.DeploymentRun, error) {
	lifecycle, err := h.lifecycle.ProjectLifecycle(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ConfirmName) != lifecycle.Name {
		return nil, domain.ErrConfirmNameMismatch
	}
	if err := requireActive(lifecycle); err != nil {
		return nil, err
	}
	if lifecycle.ProductID != "" {
		return nil, domain.ErrProjectBoundToProduct
	}
	clusterID, err := kernel.ParseClusterInfoID(strings.TrimSpace(input.ClusterID))
	if err != nil {
		return nil, domain.ErrDeploymentClusterNotFound
	}
	return h.start(ctx, wsID, memberID, projectID, clusterID, model.UndeployOptions{
		DeleteConfig: input.DeleteConfig, DeleteStorage: input.DeleteStorage,
	})
}

// StartForArchive is used by the archive flow, which has already confirmed the
// project name and moved the project into the archiving status.
func (h *UndeployHandler) StartForArchive(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
) (*model.DeploymentRun, error) {
	lifecycle, err := h.lifecycle.ProjectLifecycle(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if lifecycle.Status != projectStatusActive && lifecycle.Status != projectStatusArchiving {
		return nil, domain.ErrProjectNotActive
	}
	return h.start(ctx, wsID, memberID, projectID, clusterID, model.UndeployOptions{})
}

func (h *UndeployHandler) start(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	opts model.UndeployOptions,
) (*model.DeploymentRun, error) {
	active, err := h.repo.HasActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}
	runs, err := h.repo.ListByProject(ctx, projectID, undeployBaselineWindow)
	if err != nil {
		return nil, err
	}
	baseline := model.LatestRunByCluster(workspaceRuns(runs, wsID))[clusterID]
	if !model.IsOnlineRun(baseline) {
		return nil, domain.ErrEnvironmentNotDeployed
	}
	run := model.NewUndeployRun(wsID, projectID, memberID, baseline, opts)
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

func workspaceRuns(runs []*model.DeploymentRun, wsID kernel.WorkspaceID) []*model.DeploymentRun {
	out := make([]*model.DeploymentRun, 0, len(runs))
	for _, run := range runs {
		if run != nil && run.WorkspaceID == wsID {
			out = append(out, run)
		}
	}
	return out
}

func requireActive(lifecycle port.ProjectLifecycle) error {
	switch lifecycle.Status {
	case projectStatusActive, "":
		return nil
	case projectStatusArchiving:
		return domain.ErrProjectArchiving
	default:
		return domain.ErrProjectNotActive
	}
}

func checkLifecycle(
	ctx context.Context,
	reader port.ProjectLifecycleReader,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	if reader == nil {
		return nil
	}
	lifecycle, err := reader.ProjectLifecycle(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	return requireActive(lifecycle)
}
