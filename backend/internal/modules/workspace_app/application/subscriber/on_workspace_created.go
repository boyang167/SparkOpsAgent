package subscriber

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type OnWorkspaceCreated struct {
	db *gorm.DB
}

func NewOnWorkspaceCreated(db *gorm.DB) *OnWorkspaceCreated {
	return &OnWorkspaceCreated{db: db}
}

func (s *OnWorkspaceCreated) Handle(ctx context.Context, event events.Event) error {
	created, ok := event.(integration.WorkspaceCreated)
	if !ok {
		return nil
	}
	return command.SeedWorkspaceDefaultTemplates(
		ctx,
		s.db,
		created.WorkspaceID.String(),
		created.OwnerID.String(),
	)
}
