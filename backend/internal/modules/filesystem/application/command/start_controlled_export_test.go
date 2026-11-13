package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
)

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

type exportAuditRecorder struct{ entries []audit.Entry }

func (r *exportAuditRecorder) Log(_ context.Context, entry audit.Entry) error {
	r.entries = append(r.entries, entry)
	return nil
}
