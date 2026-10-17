package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type deploymentOperationPublisher struct {
	writer messaging.OperationWriter
	tx     postgres.Transactor
	owner  string
}

func (p deploymentOperationPublisher) enabled() bool {
	return p.writer != nil && p.tx != nil && p.owner != ""
}

func (p deploymentOperationPublisher) schedule(ctx context.Context, run *model.DeploymentRun) error {
	target := run.ID.String()
	return p.writer.Schedule(ctx, messaging.OperationRequest{
		Owner: p.owner, OperationType: outbound.OperationTypeDeployQueuePublish,
		IdempotencyKey: target + ":" + p.owner + ":" + outbound.OperationTypeDeployQueuePublish + ":" + target,
		Payload: outbound.DeployQueuePayload{
			EventID: target, RunID: target, WorkspaceID: run.WorkspaceID.String(),
		},
	})
}
