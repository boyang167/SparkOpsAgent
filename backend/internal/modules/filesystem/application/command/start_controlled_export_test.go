package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
)

type exportAuditRecorder struct {
	entries []audit.Entry
}

func (r *exportAuditRecorder) Log(_ context.Context, entry audit.Entry) error {
	r.entries = append(r.entries, entry)
	return nil
}

func TestControlledExportAuditRecordsEachWorkspaceScope(t *testing.T) {
	recorder := &exportAuditRecorder{}
	handler := &StartControlledExportHandler{audit: recorder}
	task := model.ExportTask{
		ID:                   uuid.New(),
		ActorID:              uuid.New(),
		SourceOrganizationID: uuid.New(),
		TargetOrganizationID: uuid.New(),
		SourceWorkspaceID:    uuid.New(),
		TargetWorkspaceID:    uuid.New(),
		SourceNodeID:         uuid.New(),
	}

	require.NoError(t, handler.auditBoth(context.Background(), task))
	require.Len(t, recorder.entries, 2)
	require.Equal(t, task.SourceOrganizationID, recorder.entries[0].OrgID.UUID)
	require.Equal(t, task.SourceWorkspaceID.String(), recorder.entries[0].Detail["workspace_id"])
	require.Equal(t, task.TargetOrganizationID, recorder.entries[1].OrgID.UUID)
	require.Equal(t, task.TargetWorkspaceID.String(), recorder.entries[1].Detail["workspace_id"])
}

func TestControlledExportRecordsTaskWithoutGrant(t *testing.T) {
	store := newCapabilityStoreStub()
	audits := &exportAuditRecorder{}
	handler := NewStartControlledExportHandler(store, audits)
	orgA, orgB := uuid.New(), uuid.New()
	commandID := uuid.New()
	sourceNode := uuid.New()
	cmd := StartControlledExport{
		ID: commandID, ActorID: uuid.New(),
		SourceOrganizationID: orgA, TargetOrganizationID: orgB,
		SourceWorkspaceID: uuid.New(), TargetWorkspaceID: uuid.New(),
		SourceNodeID: sourceNode,
	}
	task, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, commandID, task.ID)
	require.Equal(t, model.ExportStatusPending, task.Status)
	require.Nil(t, task.TargetNodeID)
	require.Equal(t, sourceNode, task.SourceNodeID)
	require.Len(t, audits.entries, 2)
	require.NotEqual(t, audits.entries[0].OrgID.UUID, audits.entries[1].OrgID.UUID)

	replayed, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, task.ID, replayed.ID)
	require.Len(t, audits.entries, 2)
	require.Len(t, store.exports, 1)
}
