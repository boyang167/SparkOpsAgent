package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func logAudit(logger audit.Logger, ctx context.Context, entry audit.Entry) {
	if logger == nil {
		return
	}
	_ = logger.Log(ctx, entry)
}

func orgAuditEntry(actorID kernel.MemberID, action, resourceID string, orgID kernel.OrgID, detail map[string]any) audit.Entry {
	return audit.Entry{
		ActorID: actorID, Action: action,
		ResourceType: "organization", ResourceID: resourceID,
		OrgID: &orgID, Detail: detail,
	}
}
