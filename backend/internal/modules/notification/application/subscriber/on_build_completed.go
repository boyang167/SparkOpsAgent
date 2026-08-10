package subscriber

import (
	"context"
	"fmt"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnBuildCompleted struct {
	create *command.CreateNotificationHandler
}

func NewOnBuildCompleted(create *command.CreateNotificationHandler) *OnBuildCompleted {
	return &OnBuildCompleted{create: create}
}

func (s *OnBuildCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.BuildCompleted)
	if !ok {
		return nil
	}

	link := fmt.Sprintf("/delivery/builds/%s", ev.BuildID.String())
	title := fmt.Sprintf("项目「%s」构建 #%d 完成", ev.ProjectName, ev.BuildNumber)
	body := fmt.Sprintf("状态：%s", ev.Status)

	_, err := s.create.Handle(
		ctx,
		ev.MemberID,
		ev.WorkspaceID,
		model.NotificationTypeBuildCompleted,
		title,
		body,
		link,
	)
	return err
}
