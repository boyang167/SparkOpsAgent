package operationlog

import (
	"context"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

func (s *Store) CreatePluginAudit(ctx context.Context, event sparkplugin.PluginAuditRecord) error {
	return s.Create(ctx, Record{
		Method:      "AUDIT",
		Path:        event.Action,
		Status:      200,
		Body:        event.DetailJSON,
		MemberID:    event.MemberID,
		WorkspaceID: event.WorkspaceID,
	})
}
