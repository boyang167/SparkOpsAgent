package subscriber

import (
	"context"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnBuildCompleted struct {
	scheduler *engine.Scheduler
}

func NewOnBuildCompleted(scheduler *engine.Scheduler) *OnBuildCompleted {
	return &OnBuildCompleted{scheduler: scheduler}
}

func (s *OnBuildCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.BuildCompleted)
	if !ok {
		return nil
	}
	if messaging.IsInboxConsumerContext(ctx) {
		return messaging.ErrOperationOutboxRequired
	}
	return s.scheduler.HandleBuildCompleted(ctx, ev.BuildID, ev.Status)
}
