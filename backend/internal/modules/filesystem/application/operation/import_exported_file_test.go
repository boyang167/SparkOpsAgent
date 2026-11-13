package operation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type importStore struct {
	task model.ExportTask
}

func (s *importStore) SaveCapability(context.Context, model.CapabilityRecord) error {
	return nil
}
func (s *importStore) GetCapability(context.Context, uuid.UUID) (model.CapabilityRecord, error) {
	return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
}
func (s *importStore) GetCapabilityByHash(context.Context, string) (model.CapabilityRecord, error) {
	return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
}
func (s *importStore) RevokeCapability(context.Context, uuid.UUID, time.Time) error {
	return domain.ErrCapabilityNotFound
}
func (s *importStore) SaveExportTask(context.Context, model.ExportTask) error { return nil }
func (s *importStore) GetExportTask(context.Context, uuid.UUID) (model.ExportTask, error) {
	return s.task, nil
}
func (s *importStore) MarkExportImported(_ context.Context, id, targetNodeID uuid.UUID) error {
	if s.task.ID != id {
		return domain.ErrExportNotFound
	}
	if s.task.TargetNodeID != nil && *s.task.TargetNodeID != targetNodeID {
		return domain.ErrIdempotencyConflict
	}
	s.task.TargetNodeID = &targetNodeID
	s.task.Status = model.ExportStatusImported
	return nil
}

var _ repository.CapabilityRepository = (*importStore)(nil)

type nodeCreatorStub struct {
	creates int
	nodes   []*model.Node
}

func (s *nodeCreatorStub) CreateNode(_ context.Context, node *model.Node) error {
	s.creates++
	copied := *node
	s.nodes = append(s.nodes, &copied)
	return nil
}

func TestImportCreatesNewFileIdentity(t *testing.T) {
	sourceNode := uuid.New()
	store := &importStore{task: model.ExportTask{
		ID: uuid.New(), SourceOrganizationID: uuid.New(), TargetOrganizationID: uuid.New(),
		SourceNodeID: sourceNode, Status: model.ExportStatusPending,
	}}
	nodes := &nodeCreatorStub{}
	handler := NewImportExportedFileHandler(nodes, store)
	cmd := ImportExportedFile{
		ExportID: store.task.ID, TargetNamespaceID: uuid.New(), TargetParentID: uuid.New(),
		OwnerID: uuid.New(), Name: "copied.txt",
	}
	first, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NotEqual(t, sourceNode, first.NodeID)
	require.Equal(t, 1, nodes.creates)
	require.Nil(t, nodes.nodes[0].CurrentRevisionID)
	require.NotEqual(t, sourceNode, nodes.nodes[0].ID)

	second, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, first.NodeID, second.NodeID)
	require.Equal(t, 1, nodes.creates)
	require.Equal(t, model.ExportStatusImported, store.task.Status)
}
