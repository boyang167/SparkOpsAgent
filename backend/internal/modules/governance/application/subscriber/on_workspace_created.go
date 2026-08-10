package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type OnWorkspaceCreated struct {
	bootstrap *command.BootstrapWorkspaceHandler
}

func NewOnWorkspaceCreated(bootstrap *command.BootstrapWorkspaceHandler) *OnWorkspaceCreated {
	return &OnWorkspaceCreated{bootstrap: bootstrap}
}

func (s *OnWorkspaceCreated) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(integration.WorkspaceCreated)
	if !ok {
		return nil
	}
	return s.bootstrap.Handle(ctx, ev.WorkspaceID, ev.OwnerID, authz.DefaultAdminPermissions)
}
