package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type listTransfersRepoStub struct {
	workspaceID uuid.UUID
	status      *model.TransferStatus
	limit       int
	cursor      string
	rows        []*model.TransferSession
	next        string
	err         error
}

func (s *listTransfersRepoStub) ListTransfers(
	_ context.Context,
	workspaceID uuid.UUID,
	status *model.TransferStatus,
	limit int,
	cursor string,
) ([]*model.TransferSession, string, error) {
	s.workspaceID, s.status, s.limit, s.cursor = workspaceID, status, limit, cursor
	return s.rows, s.next, s.err
}

func TestListTransfersScopesWorkspaceAndStatus(t *testing.T) {
	workspaceID := uuid.New()
	session := listTransferSessionFixture(t, workspaceID)
	repo := &listTransfersRepoStub{rows: []*model.TransferSession{session}}
	handler := NewListTransfersHandler(repo)

	rows, next, err := handler.Handle(context.Background(), ListTransfers{
		WorkspaceID: workspaceID,
		Status:      model.TransferFailed,
		Limit:       20,
	})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, workspaceID, rows[0].WorkspaceID())
	require.Empty(t, next)
	require.Equal(t, workspaceID, repo.workspaceID)
	require.Equal(t, model.TransferFailed, *repo.status)
	require.Equal(t, 20, repo.limit)
}

func TestListTransfersRejectsInvalidInputAndForeignRows(t *testing.T) {
	workspaceID := uuid.New()
	handler := NewListTransfersHandler(&listTransfersRepoStub{
		rows: []*model.TransferSession{listTransferSessionFixture(t, uuid.New())},
	})

	_, _, err := handler.Handle(context.Background(), ListTransfers{
		WorkspaceID: workspaceID,
		Status:      model.TransferStatus("UNKNOWN"),
		Limit:       20,
	})
	require.ErrorIs(t, err, model.ErrInvalidTransfer)

	_, _, err = handler.Handle(context.Background(), ListTransfers{
		WorkspaceID: workspaceID,
		Limit:       20,
	})
	require.ErrorIs(t, err, model.ErrInvalidTransfer)
}

func TestListTransfersClampsLimitToOneHundred(t *testing.T) {
	for _, test := range []struct {
		name      string
		requested int
		expected  int
	}{
		{name: "default", requested: 0, expected: 100},
		{name: "minimum", requested: 1, expected: 1},
		{name: "maximum", requested: 100, expected: 100},
		{name: "clamped", requested: 500, expected: 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &listTransfersRepoStub{}
			handler := NewListTransfersHandler(repo)
			_, _, err := handler.Handle(context.Background(), ListTransfers{
				WorkspaceID: uuid.New(),
				Limit:       test.requested,
				Cursor:      "opaque-cursor",
			})
			require.NoError(t, err)
			require.Equal(t, test.expected, repo.limit)
			require.Equal(t, "opaque-cursor", repo.cursor)
		})
	}
}

func TestListTransfersPropagatesRepositoryError(t *testing.T) {
	expected := errors.New("list failed")
	handler := NewListTransfersHandler(&listTransfersRepoStub{err: expected})

	_, _, err := handler.Handle(context.Background(), ListTransfers{
		WorkspaceID: uuid.New(),
		Limit:       20,
	})

	require.ErrorIs(t, err, expected)
}

func listTransferSessionFixture(t *testing.T, workspaceID uuid.UUID) *model.TransferSession {
	t.Helper()
	parentID := uuid.New()
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID:                 uuid.New(),
		WorkspaceID:        workspaceID,
		NamespaceID:        uuid.New(),
		TargetParentID:     &parentID,
		TargetName:         "payload.bin",
		CreatedBy:          uuid.New(),
		Protocol:           model.TransferProtocolDirect,
		ExpectedSize:       1,
		IdempotencyKey:     uuid.NewString(),
		RequestHash:        strings64("d"),
		TemporaryObjectKey: "transfers/payload",
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
		Classification:     model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	return session
}
