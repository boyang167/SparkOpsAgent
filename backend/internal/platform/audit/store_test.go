package audit

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestGormStoreListFiltersWorkspaceAndDecisionBeforeLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit-scope?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	store := NewGormStore(db)
	require.NoError(t, store.AutoMigrate())

	orgID := uuid.New()
	workspaceA := uuid.New()
	workspaceB := uuid.New()
	actorID := kernel.MemberID{UUID: uuid.New()}
	for _, fixture := range []struct {
		workspace uuid.UUID
		decision  string
	}{
		{workspaceB, "DENY"},
		{workspaceA, "ALLOW"},
		{workspaceA, "DENY"},
	} {
		require.NoError(t, store.Append(context.Background(), Entry{
			ActorID: actorID, OrgID: &kernel.OrgID{UUID: orgID},
			Action: "filesystem.node.rename", ResourceType: "filesystem", ResourceID: uuid.NewString(),
			Detail: map[string]any{
				"workspace_id": fixture.workspace.String(),
				"decision":     fixture.decision,
			},
		}))
	}

	rows, err := store.List(context.Background(), ListFilter{
		OrgID: &kernel.OrgID{UUID: orgID}, WorkspaceID: workspaceA.String(),
		Decision: "DENY", Limit: 1,
	})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, workspaceA.String(), rows[0].Detail["workspace_id"])
	require.Equal(t, "DENY", rows[0].Detail["decision"])
}
