package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
)

type auditListStub struct {
	filter audit.ListFilter
	rows   []audit.LogView
}

func (s *auditListStub) List(_ context.Context, filter audit.ListFilter) ([]audit.LogView, error) {
	s.filter = filter
	return s.rows, nil
}

func TestListFileAuditEventsScopesWorkspaceAndDecisionBeforePagination(t *testing.T) {
	workspaceID := uuid.New()
	store := &auditListStub{rows: []audit.LogView{{
		ID: "event-1", ActorID: uuid.NewString(), Action: "filesystem.node.rename",
		ResourceType: "filesystem", ResourceID: uuid.NewString(),
		Detail:    map[string]any{"workspace_id": workspaceID.String(), "decision": "DENY"},
		CreatedAt: "2026-11-14T04:00:00Z",
	}}}

	result, err := NewListFileAuditEventsHandler(store).Handle(context.Background(), ListFileAuditEvents{
		OrganizationID: uuid.New(),
		WorkspaceID:    workspaceID,
		Result:         "deny",
		Limit:          20,
	})

	require.NoError(t, err)
	require.Equal(t, workspaceID.String(), store.filter.WorkspaceID)
	require.Equal(t, "DENY", store.filter.Decision)
	require.Len(t, result.Items, 1)
	require.Equal(t, "DENY", result.Items[0].Result)
}
