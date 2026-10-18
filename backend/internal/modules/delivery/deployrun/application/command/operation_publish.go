package command

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type deploymentOperationPublisher struct {
	writer     messaging.OperationWriter
	tx         postgres.Transactor
	owner      string
	repeatable bool
}

type OperationRunPublisher struct {
	publisher deploymentOperationPublisher
}

func NewOperationRunPublisher(writer messaging.OperationWriter, tx postgres.Transactor) *OperationRunPublisher {
	return &OperationRunPublisher{publisher: deploymentOperationPublisher{
		writer: writer, tx: tx, owner: "delivery.reconciler", repeatable: true,
	}}
}

func (p *OperationRunPublisher) PublishRun(ctx context.Context, run *model.DeploymentRun) error {
	if p == nil || !p.publisher.enabled() {
		return fmt.Errorf("deployment operation publisher is not configured")
	}
	return p.publisher.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		return p.publisher.schedule(txCtx, run)
	})
}

func (p deploymentOperationPublisher) enabled() bool {
	return p.writer != nil && p.tx != nil && p.owner != ""
}

func (p deploymentOperationPublisher) schedule(ctx context.Context, run *model.DeploymentRun) error {
	target := run.ID.String()
	eventID, owner := target, p.owner
	if metadata, ok := events.MetadataFromContext(ctx); ok {
		eventID, owner = metadata.EventID, metadata.Subscriber
	}
	return p.writer.Schedule(ctx, messaging.OperationRequest{
		Owner: owner, OperationType: outbound.OperationTypeDeployQueuePublish,
		IdempotencyKey: eventID + ":" + owner + ":" + outbound.OperationTypeDeployQueuePublish + ":" + target,
		Repeatable:     p.repeatable,
		Payload: outbound.DeployQueuePayload{
			EventID: eventID, RunID: target, WorkspaceID: run.WorkspaceID.String(),
		},
	})
}
