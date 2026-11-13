package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type stubVerify struct{}

func (stubVerify) Handle(context.Context, command.VerifyTransfer) error { return nil }

type recordingCommit struct {
	calls int
}

func (r *recordingCommit) Handle(context.Context, command.CommitTransfer) (command.CommitTransferResult, error) {
	r.calls++
	return command.CommitTransferResult{}, nil
}

type sessionStatusReader struct {
	status model.TransferStatus
}

func (r sessionStatusReader) GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error) {
	return restoreSessionWithStatus(r.status)
}

func restoreSessionWithStatus(status model.TransferStatus) (*model.TransferSession, error) {
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: uuid.New(), WorkspaceID: uuid.New(), NamespaceID: uuid.New(), CreatedBy: uuid.New(),
		TargetName: "file.bin", Protocol: model.TransferProtocolDirect, ExpectedSize: 1,
		IdempotencyKey: "key", RequestHash: stringsRepeat64('a'), TemporaryObjectKey: "tmp/obj",
		ExpiresAt: time.Now().Add(time.Hour), Classification: model.TransferClassificationInternal,
	})
	if err != nil {
		return nil, err
	}
	return model.RestoreTransferSession(model.PersistedTransferSession{
		Input: model.TransferSessionInput{
			ID: session.ID(), WorkspaceID: session.WorkspaceID(), NamespaceID: session.NamespaceID(),
			CreatedBy: session.CreatedBy(), TargetName: session.TargetName(), Protocol: session.Protocol(),
			ExpectedSize: session.ExpectedSize(), IdempotencyKey: session.IdempotencyKey(),
			RequestHash: session.RequestHash(), TemporaryObjectKey: session.TemporaryObjectKey(),
			ExpiresAt: session.ExpiresAt(), Classification: session.Classification(),
		},
		Status: status, Version: 1,
	})
}

func stringsRepeat64(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestOnTransferUploadedSkipsCommitWhenRejected(t *testing.T) {
	commit := &recordingCommit{}
	s := NewOnTransferUploaded(stubVerify{}, commit, sessionStatusReader{status: model.TransferRejected})
	event := filesystemevent.TransferUploaded{
		BaseEvent: events.NewBaseEvent(filesystemevent.TransferUploadedName),
		SessionID: uuid.New(), WorkspaceID: uuid.New(),
	}
	require.NoError(t, s.Handle(context.Background(), event))
	require.Equal(t, 0, commit.calls)
}

func TestOnTransferUploadedCommitWhenScanning(t *testing.T) {
	commit := &recordingCommit{}
	s := NewOnTransferUploaded(stubVerify{}, commit, sessionStatusReader{status: model.TransferScanning})
	event := filesystemevent.TransferUploaded{
		BaseEvent: events.NewBaseEvent(filesystemevent.TransferUploadedName),
		SessionID: uuid.New(), WorkspaceID: uuid.New(),
	}
	require.NoError(t, s.Handle(context.Background(), event))
	require.Equal(t, 1, commit.calls)
}
