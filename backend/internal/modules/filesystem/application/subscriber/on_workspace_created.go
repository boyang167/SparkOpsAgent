package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type OnWorkspaceCreated struct {
	ensure *command.EnsureNamespaceHandler
}

func NewOnWorkspaceCreated(ensure *command.EnsureNamespaceHandler) *OnWorkspaceCreated {
	return &OnWorkspaceCreated{ensure: ensure}
}

func (s *OnWorkspaceCreated) Handle(ctx context.Context, event events.Event) error {
	created, ok := event.(integration.WorkspaceCreated)
	if !ok {
		return nil
	}
	_, err := s.ensure.Handle(ctx, command.EnsureNamespace{
		ActorID:        created.OwnerID.UUID,
		OrganizationID: created.OrgID.UUID,
		WorkspaceID:    created.WorkspaceID.UUID,
		NamespaceType:  model.NamespaceTypeWorkspace,
	})
	return err
}
