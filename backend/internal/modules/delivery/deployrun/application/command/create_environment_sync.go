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
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateEnvironmentSyncDeploymentInput struct {
	SnapshotID kernel.EnvironmentSyncSnapshotID
	ProjectID  kernel.ProjectID
}

type CreateEnvironmentSyncDeploymentHandler struct {
	runs         repository.DeploymentRunRepository
	snapshots    repository.EnvironmentSyncSnapshotRepository
	environments port.EnvironmentResolver
	queue        messaging.Queue
	clock        func() time.Time
	operations   deploymentOperationPublisher
}

func NewCreateEnvironmentSyncDeploymentHandler(
	runs repository.DeploymentRunRepository,
	snapshots repository.EnvironmentSyncSnapshotRepository,
	environments port.EnvironmentResolver,
	queue messaging.Queue,
	clock func() time.Time,
) *CreateEnvironmentSyncDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	return &CreateEnvironmentSyncDeploymentHandler{
		runs: runs, snapshots: snapshots, environments: environments, queue: queue, clock: clock,
	}
}

func (h *CreateEnvironmentSyncDeploymentHandler) UseOperationOutbox(writer messaging.OperationWriter, tx postgres.Transactor) {
	h.operations = deploymentOperationPublisher{writer: writer, tx: tx, owner: "delivery.command.create-environment-sync"}
}

func (h *CreateEnvironmentSyncDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	input CreateEnvironmentSyncDeploymentInput,
) (*model.DeploymentRun, error) {
	if h.operations.enabled() {
		var result *model.DeploymentRun
		err := h.operations.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			var err error
			result, err = h.handle(txCtx, wsID, memberID, input)
			return err
		})
		return result, err
	}
	return h.handle(ctx, wsID, memberID, input)
}

func (h *CreateEnvironmentSyncDeploymentHandler) handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	input CreateEnvironmentSyncDeploymentInput,
) (*model.DeploymentRun, error) {
	snapshot, err := h.snapshots.FindByID(ctx, input.SnapshotID)
	if err != nil {
		return nil, err
	}
	if snapshot.BoundRunID == nil {
		return nil, domain.ErrEnvironmentSyncSnapshotNotBound
	}
	if snapshot.WorkspaceID != wsID {
		return nil, domain.ErrEnvironmentSyncSnapshotNotFound
	}

	var projectSnapshot *model.EnvironmentSyncProjectSnapshot
	for i := range snapshot.Projects {
		if snapshot.Projects[i].ProjectID == input.ProjectID {
			projectSnapshot = &snapshot.Projects[i]
			break
		}
	}
	if projectSnapshot == nil {
		return nil, domain.ErrEnvironmentSyncProjectNotFound
	}

	target, err := h.environments.Resolve(ctx, wsID, input.ProjectID, snapshot.TargetEnvironmentID)
	if err != nil {
		return nil, err
	}
	active, err := h.runs.HasActiveByProject(ctx, input.ProjectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}

	manifests := append([]model.SyncManifest(nil), projectSnapshot.Plan.Manifests...)
	if err := model.ValidateSyncManifests(manifests, target.Namespace); err != nil {
		return nil, err
	}

	run := model.NewEnvironmentSyncDeploymentRun(wsID, input.ProjectID, target.ClusterID, memberID)
	run.EnvironmentID = &target.EnvironmentID
	run.SourceEnvironmentID = &snapshot.SourceEnvironmentID
	run.EnvironmentSyncSnapshotID = &snapshot.ID
	run.DeployNamespace = target.Namespace
	run.SyncManifests = manifests
	run.ImageRepository = strings.TrimSpace(projectSnapshot.Plan.ImageRepository)
	run.ImageTag = strings.TrimSpace(projectSnapshot.Plan.ImageTag)
	run.CreatedAt = h.clock().UTC()

	if err := h.runs.Create(ctx, run); err != nil {
		return nil, err
	}
	if h.operations.enabled() {
		if err := h.operations.schedule(ctx, run); err != nil {
			return nil, err
		}
		return run, nil
	}
	payload, err := json.Marshal(consumer.Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})
	if err != nil {
		return nil, fmt.Errorf("marshal deployment message: %w", err)
	}
	if err := h.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
		run.Fail("publish deploy message: "+err.Error(), h.clock())
		if updateErr := h.runs.Update(ctx, run); updateErr != nil {
			return nil, errors.Join(err, fmt.Errorf("mark deployment failed: %w", updateErr))
		}
		return nil, err
	}
	return run, nil
}
