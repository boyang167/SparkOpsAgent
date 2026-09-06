package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type OnWorkspaceCreated struct {
	bootstrap    *command.BootstrapWorkspaceHandler
	menuPolicies *command.ApplyMenuRolePoliciesHandler
}

func NewOnWorkspaceCreated(
	bootstrap *command.BootstrapWorkspaceHandler,
	menuPolicies *command.ApplyMenuRolePoliciesHandler,
) *OnWorkspaceCreated {
	return &OnWorkspaceCreated{bootstrap: bootstrap, menuPolicies: menuPolicies}
}

func (s *OnWorkspaceCreated) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(integration.WorkspaceCreated)
	if !ok {
		return nil
	}
	if err := s.bootstrap.Handle(ctx, ev.WorkspaceID, ev.OwnerID, authz.DefaultAdminPermissions); err != nil {
		return err
	}
	if s.menuPolicies == nil {
		return nil
	}
	return s.menuPolicies.Handle(ctx, ev.WorkspaceID, command.ApplyMenuRolePoliciesOptions{SkipIfConfigured: true})
}
