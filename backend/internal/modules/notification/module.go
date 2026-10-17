package notification

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/persistence"
	notificationhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/notification/interface/http"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "notification" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("notification module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("notification module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("notification migrate: %w", err)
	}

	repo := persistence.NewNotificationRepository(c.DB)
	create := command.NewCreateNotificationHandler(repo)
	markRead := command.NewMarkReadHandler(repo)
	markAllRead := command.NewMarkAllReadHandler(repo)
	list := query.NewListNotificationsHandler(repo)
	agentACL := acl.NewAgentACL(c.DB)

	if err := c.EventBus.Subscribe(event.RunCompletedName, "notification.run-completed", subscriber.NewOnRunCompleted(create, agentACL).Handle); err != nil {
		return fmt.Errorf("subscribe notification run completed: %w", err)
	}
	if err := c.EventBus.Subscribe(workflowevent.WorkflowRunCompletedName, "notification.workflow-run-completed", subscriber.NewOnWorkflowRunCompleted(create).Handle); err != nil {
		return fmt.Errorf("subscribe notification workflow run completed: %w", err)
	}
	if err := c.EventBus.Subscribe(deliveryevent.BuildCompletedName, "notification.build-completed", subscriber.NewOnBuildCompleted(create).Handle); err != nil {
		return fmt.Errorf("subscribe notification build completed: %w", err)
	}

	handler := notificationhttp.NewHandler(list, markRead, markAllRead)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	// Notifications are scoped to the calling member, so workspace membership is the only gate.
	c.Engine.GET("/api/v1/notifications", auth, ws, wsMember, handler.ListNotifications)
	c.Engine.PATCH("/api/v1/notifications/:notificationId/read", auth, ws, wsMember, handler.MarkRead)
	c.Engine.POST("/api/v1/notifications/read-all", auth, ws, wsMember, handler.MarkAllRead)

	return nil
}
