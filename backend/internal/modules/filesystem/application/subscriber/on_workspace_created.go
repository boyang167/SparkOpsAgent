package subscriber

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type OnWorkspaceCreated struct {
	create *command.CreateNamespaceHandler
}

func NewOnWorkspaceCreated(create *command.CreateNamespaceHandler) *OnWorkspaceCreated {
	return &OnWorkspaceCreated{create: create}
}

func (s *OnWorkspaceCreated) Handle(ctx context.Context, event events.Event) error {
	created, ok := event.(integration.WorkspaceCreated)
	if !ok {
		return nil
	}
	_, err := s.create.Handle(ctx, command.CreateNamespace{
		ActorID:        created.OwnerID.UUID,
		OrganizationID: created.OrgID.UUID,
		WorkspaceID:    created.WorkspaceID.UUID,
		Authorization: command.Authorization{
			Allowed: true, WorkspaceID: created.WorkspaceID.UUID,
			ResourceID: created.WorkspaceID.UUID, Permission: model.ResourcePermissionAdmin,
		},
	})
	return err
}
