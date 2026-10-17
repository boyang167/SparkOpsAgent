package subscriber

import (
	"context"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnBuildCompleted struct {
	scheduler  *engine.Scheduler
	operations messaging.OperationWriter
}

func NewOnBuildCompleted(scheduler *engine.Scheduler, writers ...messaging.OperationWriter) *OnBuildCompleted {
	handler := &OnBuildCompleted{scheduler: scheduler}
	if len(writers) > 0 {
		handler.operations = writers[0]
	}
	return handler
}

func (s *OnBuildCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.BuildCompleted)
	if !ok {
		return nil
	}
	if s.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return nil
		}
		return s.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: workflowoutbound.OperationTypeBuildTrigger,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + workflowoutbound.OperationTypeBuildTrigger + ":" + ev.BuildID.String(),
			Payload:        workflowoutbound.CompletionPayload{EventID: metadata.EventID, TargetID: ev.BuildID.String(), Status: ev.Status},
		})
	}
	return s.scheduler.HandleBuildCompleted(ctx, ev.BuildID, ev.Status)
}
