package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OnWorkflowRunCompleted struct {
	releasePlans *service.ReleasePlanExecutor
	sprintItems  *service.SprintWorkItemExecutor
	operations   messaging.OperationWriter
}

func NewOnWorkflowRunCompleted(
	releasePlans *service.ReleasePlanExecutor,
	sprintItems *service.SprintWorkItemExecutor,
	writers ...messaging.OperationWriter,
) *OnWorkflowRunCompleted {
	handler := &OnWorkflowRunCompleted{releasePlans: releasePlans, sprintItems: sprintItems}
	if len(writers) > 0 {
		handler.operations = writers[0]
	}
	return handler
}

func (h *OnWorkflowRunCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(pwfevent.WorkflowRunCompleted)
	if !ok {
		return nil
	}
	if h.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return nil
		}
		return h.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: productoutbound.OperationTypeWorkflowTrigger,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + productoutbound.OperationTypeWorkflowTrigger + ":" + ev.RunID.String(),
			Payload: productoutbound.WorkflowCompletionPayload{
				EventID: metadata.EventID, RunID: ev.RunID.String(), Status: ev.Status, ErrorMessage: ev.ErrorMessage,
			},
		})
	}
	success := ev.Status == "success"
	if h.releasePlans != nil {
		if err := h.releasePlans.OnWorkflowRunCompleted(ctx, ev.RunID, success, ev.ErrorMessage, kernel.MemberID{}); err != nil {
			return err
		}
	}
	if h.sprintItems != nil {
		if err := h.sprintItems.OnWorkflowRunCompleted(ctx, ev.RunID, success, ev.ErrorMessage); err != nil {
			return err
		}
	}
	return nil
}
