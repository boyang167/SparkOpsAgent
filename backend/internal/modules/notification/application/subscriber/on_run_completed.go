package subscriber

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnRunCompleted struct {
	create *command.CreateNotificationHandler
	agents *acl.AgentACL
}

func NewOnRunCompleted(create *command.CreateNotificationHandler, agents *acl.AgentACL) *OnRunCompleted {
	return &OnRunCompleted{create: create, agents: agents}
}

func (s *OnRunCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(event.RunCompleted)
	if !ok {
		return nil
	}

	agentName := "Agent"
	projectID := ev.ProjectID.String()
	if name, pid, err := s.agents.GetAgentInfo(ctx, ev.AgentID); err == nil {
		agentName = name
		if pid != "" {
			projectID = pid
		}
	}

	link := fmt.Sprintf(
		"/projects/%s/agents/%s?conversationId=%s",
		projectID, ev.AgentID.String(), ev.ConversationID.String(),
	)
	title := fmt.Sprintf("「%s」对话已完成", agentName)
	body := "点击查看对话结果"

	_, err := s.create.Handle(
		ctx,
		ev.MemberID,
		ev.WorkspaceID,
		model.NotificationTypeRunCompleted,
		title,
		body,
		link,
	)
	return err
}
