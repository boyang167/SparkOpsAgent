package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OnWorkflowRunCompleted struct {
	releasePlans *service.ReleasePlanExecutor
	sprintItems  *service.SprintWorkItemExecutor
}

func NewOnWorkflowRunCompleted(
	releasePlans *service.ReleasePlanExecutor,
	sprintItems *service.SprintWorkItemExecutor,
) *OnWorkflowRunCompleted {
	return &OnWorkflowRunCompleted{releasePlans: releasePlans, sprintItems: sprintItems}
}

func (h *OnWorkflowRunCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(pwfevent.WorkflowRunCompleted)
	if !ok {
		return nil
	}
	if messaging.IsInboxConsumerContext(ctx) {
		return messaging.ErrOperationOutboxRequired
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
