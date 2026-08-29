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

type CreateInput struct {
	ClusterID   string `json:"cluster_id"`
	GitRefType  string `json:"git_ref_type"`
	GitRefValue string `json:"git_ref_value"`
	ImageTag    string `json:"image_tag"`
}

type CreateDeploymentHandler struct {
	repo   repository.DeploymentRunRepository
	reader port.ProjectDeploymentReader
	queue  messaging.Queue
	clock  func() time.Time
}

func NewCreateDeploymentHandler(
	repo repository.DeploymentRunRepository,
	reader port.ProjectDeploymentReader,
	queue messaging.Queue,
	clock func() time.Time,
) *CreateDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	return &CreateDeploymentHandler{repo: repo, reader: reader, queue: queue, clock: clock}
}

func (h *CreateDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input CreateInput,
) (*model.DeploymentRun, error) {
	var selected kernel.ClusterInfoID
	var err error
	if strings.TrimSpace(input.ClusterID) != "" {
		selected, err = kernel.ParseClusterInfoID(input.ClusterID)
		if err != nil {
			return nil, domain.ErrDeploymentClusterNotFound
		}
	}
	deploymentContext, err := h.reader.Load(ctx, wsID, projectID, selected)
	if err != nil {
		return nil, err
	}
	active, err := h.repo.HasActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}

	run := model.NewDeploymentRun(wsID, projectID, deploymentContext.Cluster.ID, memberID,
		input.GitRefType, input.GitRefValue, input.ImageTag)
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

type AbortDeploymentHandler struct {
	repo         repository.DeploymentRunRepository
	buildAborter BuildAborter
	runCanceler  RunCanceler
	clock        func() time.Time
}

type BuildAborter interface {
	Handle(context.Context, kernel.WorkspaceID, kernel.BuildID) error
}

type RunCanceler interface {
	Cancel(string) bool
}

type AbortDependencies struct {
	Builds BuildAborter
	Runs   RunCanceler
}

func NewAbortDeploymentHandler(
	repo repository.DeploymentRunRepository,
	clock func() time.Time,
	dependencies ...AbortDependencies,
) *AbortDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	handler := &AbortDeploymentHandler{repo: repo, clock: clock}
	if len(dependencies) > 0 {
		handler.buildAborter = dependencies[0].Builds
		handler.runCanceler = dependencies[0].Runs
	}
	return handler
}

func (h *AbortDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	runID kernel.DeploymentRunID,
) (*model.DeploymentRun, error) {
	run, err := h.repo.FindByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != wsID {
		return nil, domain.ErrDeploymentRunNotFound
	}
	if run.Status == model.StatusAborted || run.Status == model.StatusFailed || run.Status == model.StatusSucceeded {
		return run, nil
	}
	switch run.Status {
	case model.StatusBuilding:
		if run.BuildID == nil {
			return nil, domain.ErrDeploymentBindingInvalid
		}
		if h.buildAborter == nil {
			return nil, errors.New("deployment build aborter is unavailable")
		}
		if err := h.buildAborter.Handle(ctx, wsID, *run.BuildID); err != nil {
			return nil, err
		}
	case model.StatusPreparing, model.StatusDeploying:
		if h.runCanceler != nil {
			h.runCanceler.Cancel(run.ID.String())
		}
	}
	if err := run.Transition(model.StatusAborted, h.clock()); err != nil {
		return nil, err
	}
	run.LeaseOwner = ""
	run.LeaseExpiresAt = nil
	if err := h.repo.Update(ctx, run); err != nil {
		if errors.Is(err, domain.ErrDeploymentRunConcurrentUpdate) {
			latest, findErr := h.repo.FindByID(ctx, runID)
			if findErr != nil {
				return nil, findErr
			}
			if latest.WorkspaceID == wsID &&
				(latest.Status == model.StatusAborted || latest.Status == model.StatusFailed || latest.Status == model.StatusSucceeded) {
				return latest, nil
			}
		}
		return nil, err
	}
	return run, nil
}
