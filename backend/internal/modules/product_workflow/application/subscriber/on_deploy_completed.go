package subscriber

import (
	"context"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnDeployCompleted struct {
	scheduler  *engine.Scheduler
	operations messaging.OperationWriter
}

func NewOnDeployCompleted(scheduler *engine.Scheduler, writers ...messaging.OperationWriter) *OnDeployCompleted {
	handler := &OnDeployCompleted{scheduler: scheduler}
	if len(writers) > 0 {
		handler.operations = writers[0]
	}
	return handler
}

func (s *OnDeployCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.DeployCompleted)
	if !ok {
		return nil
	}
	if s.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return nil
		}
		return s.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: workflowoutbound.OperationTypeDeployTrigger,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + workflowoutbound.OperationTypeDeployTrigger + ":" + ev.RunID.String(),
			Payload:        workflowoutbound.CompletionPayload{EventID: metadata.EventID, TargetID: ev.RunID.String(), Status: ev.Status},
		})
	}
	return s.scheduler.HandleDeployCompleted(ctx, ev.RunID, ev.Status)
}
