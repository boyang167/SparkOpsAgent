package subscriber

import (
	"context"

	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OnWorkflowRunCompleted struct {
	executor *service.ReleasePlanExecutor
}

func NewOnWorkflowRunCompleted(executor *service.ReleasePlanExecutor) *OnWorkflowRunCompleted {
	return &OnWorkflowRunCompleted{executor: executor}
}

func (h *OnWorkflowRunCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(pwfevent.WorkflowRunCompleted)
	if !ok {
		return nil
	}
	if h.executor == nil {
		return nil
	}
	success := ev.Status == "success"
	return h.executor.OnWorkflowRunCompleted(ctx, ev.RunID, success, ev.ErrorMessage, kernel.MemberID{})
}
