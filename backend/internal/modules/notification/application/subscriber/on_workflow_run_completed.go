package subscriber

import (
	"context"
	"fmt"

	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnWorkflowRunCompleted struct {
	create *command.CreateNotificationHandler
}

func NewOnWorkflowRunCompleted(create *command.CreateNotificationHandler) *OnWorkflowRunCompleted {
	return &OnWorkflowRunCompleted{create: create}
}

func (s *OnWorkflowRunCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(workflowevent.WorkflowRunCompleted)
	if !ok {
		return nil
	}

	link := fmt.Sprintf(
		"/projects/%s/workflows/%s",
		ev.ProjectID.String(), ev.WorkflowID.String(),
	)
	title := fmt.Sprintf("Workflow「%s」运行完成", ev.WorkflowName)
	body := fmt.Sprintf("状态：%s", ev.Status)

	_, err := s.create.Handle(
		ctx,
		ev.MemberID,
		ev.WorkspaceID,
		model.NotificationTypeWorkflowRunCompleted,
		title,
		body,
		link,
	)
	return err
}
