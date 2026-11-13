package command

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	agentsign "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/capability"
)

type capabilityStoreStub struct {
	records map[uuid.UUID]model.CapabilityRecord
	exports map[uuid.UUID]model.ExportTask
}

func newCapabilityStoreStub() *capabilityStoreStub {
	return &capabilityStoreStub{
		records: map[uuid.UUID]model.CapabilityRecord{},
		exports: map[uuid.UUID]model.ExportTask{},
	}
}

func (s *capabilityStoreStub) SaveCapability(_ context.Context, record model.CapabilityRecord) error {
	if _, ok := s.records[record.ID]; ok {
		return domain.ErrIdempotencyConflict
	}
	s.records[record.ID] = record
	return nil
}

func (s *capabilityStoreStub) GetCapability(_ context.Context, id uuid.UUID) (model.CapabilityRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
	}
	return record, nil
}

func (s *capabilityStoreStub) GetCapabilityByHash(_ context.Context, hash string) (model.CapabilityRecord, error) {
	for _, record := range s.records {
		if record.TokenHash == hash {
			return record, nil
		}
	}
	return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
}

func (s *capabilityStoreStub) RevokeCapability(_ context.Context, id uuid.UUID, at time.Time) error {
	record, ok := s.records[id]
	if !ok {
		return domain.ErrCapabilityNotFound
	}
	if record.RevokedAt == nil {
		record.RevokedAt = &at
		s.records[id] = record
	}
	return nil
}

func (s *capabilityStoreStub) SaveExportTask(_ context.Context, task model.ExportTask) error {
	if _, ok := s.exports[task.ID]; ok {
		return domain.ErrIdempotencyConflict
	}
	s.exports[task.ID] = task
	return nil
}

func (s *capabilityStoreStub) GetExportTask(_ context.Context, id uuid.UUID) (model.ExportTask, error) {
	task, ok := s.exports[id]
	if !ok {
		return model.ExportTask{}, domain.ErrExportNotFound
	}
	return task, nil
}

func (s *capabilityStoreStub) MarkExportImported(_ context.Context, id, targetNodeID uuid.UUID) error {
	task, ok := s.exports[id]
	if !ok {
		return domain.ErrExportNotFound
	}
	if task.TargetNodeID != nil && *task.TargetNodeID != targetNodeID {
		return domain.ErrIdempotencyConflict
	}
	task.TargetNodeID = &targetNodeID
	task.Status = model.ExportStatusImported
	s.exports[id] = task
	return nil
}

var _ repository.CapabilityRepository = (*capabilityStoreStub)(nil)

func TestIssueCapabilityPersistsHashAndIntersectsRunScope(t *testing.T) {
	signer, err := agentsign.NewSigner("test-secret", "sparkops-agent-capability")
	require.NoError(t, err)
	store := newCapabilityStoreStub()
	fileA, fileB := uuid.New(), uuid.New()
	commandID := uuid.New()
	handler := NewIssueCapabilityHandler(service.NewCapabilityIssuer(signer), store)
	issued, err := handler.Handle(context.Background(), IssueCapability{
		CommandID: commandID, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), RunID: uuid.New(),
		ResourceIDs: []uuid.UUID{fileA, fileB}, RunResourceIDs: []uuid.UUID{fileA},
		Actions: []string{"file:read"}, RunActions: []string{"file:read"},
		Audience: "sparkops-agent-capability", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{fileA}, issued.ResourceIDs)
	require.NotEmpty(t, issued.Token)
	saved := store.records[issued.ID]
	require.NotEqual(t, issued.Token, saved.TokenHash)
	require.Len(t, saved.TokenHash, 64)
	require.NotContains(t, saved.TokenHash, ".")

	replayed, err := handler.Handle(context.Background(), IssueCapability{
		CommandID: commandID, OrganizationID: saved.OrganizationID, WorkspaceID: saved.WorkspaceID, ActorID: saved.ActorID, RunID: saved.RunID,
		ResourceIDs: []uuid.UUID{fileA, fileB}, RunResourceIDs: []uuid.UUID{fileA},
		Actions: []string{"file:read"}, Audience: "sparkops-agent-capability", ExpiresAt: saved.ExpiresAt,
	})
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Empty(t, replayed.Token)
	require.Equal(t, issued.ID, replayed.ID)
	require.Len(t, store.records, 1)
}

func TestRevokeCapabilityRejectsStoredToken(t *testing.T) {
	signer, err := agentsign.NewSigner("test-secret", "sparkops-agent-capability")
	require.NoError(t, err)
	store := newCapabilityStoreStub()
	fileA := uuid.New()
	handler := NewIssueCapabilityHandler(service.NewCapabilityIssuer(signer), store)
	issued, err := handler.Handle(context.Background(), IssueCapability{
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), RunID: uuid.New(),
		ResourceIDs: []uuid.UUID{fileA}, RunResourceIDs: []uuid.UUID{fileA},
		Actions: []string{"file:read"}, Audience: "sparkops-agent-capability", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, AuthorizeCapability(context.Background(), signer, store, issued.Token, time.Now().UTC()))
	require.NoError(t, NewRevokeCapabilityHandler(store).Handle(context.Background(), RevokeCapability{ID: issued.ID}))
	err = AuthorizeCapability(context.Background(), signer, store, issued.Token, time.Now().UTC())
	require.ErrorIs(t, err, domain.ErrCapabilityRevoked)
}
