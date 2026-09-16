package subscriber

import (
	"context"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnDeployCompleted struct {
	scheduler *engine.Scheduler
}

func NewOnDeployCompleted(scheduler *engine.Scheduler) *OnDeployCompleted {
	return &OnDeployCompleted{scheduler: scheduler}
}

func (s *OnDeployCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.DeployCompleted)
	if !ok {
		return nil
	}
	return s.scheduler.HandleDeployCompleted(ctx, ev.RunID, ev.Status)
}
