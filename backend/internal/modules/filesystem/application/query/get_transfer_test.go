package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type getTransferRepoStub struct {
	session *model.TransferSession
	parts   []model.TransferPart
}

func (s getTransferRepoStub) GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error) {
	if s.session == nil {
		return nil, domain.ErrTransferNotFound
	}
	return s.session, nil
}

func (s getTransferRepoStub) ListTransferParts(context.Context, uuid.UUID) ([]model.TransferPart, error) {
	return s.parts, nil
}

func TestGetTransferReturnsSessionAndParts(t *testing.T) {
	workspaceID, parentID, sessionID := uuid.New(), uuid.New(), uuid.New()
	providerID := uuid.New()
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: sessionID, WorkspaceID: workspaceID, NamespaceID: uuid.New(),
		TargetParentID: &parentID, TargetName: "payload.bin", CreatedBy: uuid.New(),
		Protocol: model.TransferProtocolMultipart, ExpectedSize: 10,
		RequestHash: strings64("a"), IdempotencyKey: "key-1",
		TemporaryObjectKey: "transfers/" + sessionID.String() + "/content",
		ExpiresAt: time.Now().UTC().Add(time.Hour), ProviderID: &providerID,
		Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	part, err := model.NewTransferPart(model.TransferPartInput{
		ID: uuid.New(), SessionID: sessionID, PartNumber: 1,
		Offset: 0, Size: 5, SessionSize: 10, ETag: `"etag-1"`,
	})
	require.NoError(t, err)

	get := NewGetTransferHandler(getTransferRepoStub{session: session, parts: []model.TransferPart{*part}})
	result, err := get.Handle(context.Background(), GetTransfer{
		WorkspaceID: workspaceID, SessionID: sessionID,
		Authorization: command.Authorization{
			Allowed: true, WorkspaceID: workspaceID, ResourceID: parentID,
			Permission: model.ResourcePermissionRead,
		},
	})
	require.NoError(t, err)
	require.Equal(t, sessionID, result.Session.ID())
	require.Len(t, result.Parts, 1)
	require.Equal(t, 1, result.Parts[0].PartNumber())
	require.Equal(t, int64(10), result.Descriptor.PartSize)
}

func TestGetTransferRejectsForeignWorkspace(t *testing.T) {
	workspaceID, parentID, sessionID := uuid.New(), uuid.New(), uuid.New()
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: sessionID, WorkspaceID: workspaceID, NamespaceID: uuid.New(),
		TargetParentID: &parentID, TargetName: "payload.bin", CreatedBy: uuid.New(),
		Protocol: model.TransferProtocolDirect, ExpectedSize: 1,
		RequestHash: strings64("b"), IdempotencyKey: "key-2",
		TemporaryObjectKey: "transfers/" + sessionID.String() + "/content",
		ExpiresAt: time.Now().UTC().Add(time.Hour), Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	get := NewGetTransferHandler(getTransferRepoStub{session: session})
	_, err = get.Handle(context.Background(), GetTransfer{
		WorkspaceID: uuid.New(), SessionID: sessionID,
		Authorization: command.Authorization{
			Allowed: true, WorkspaceID: workspaceID, ResourceID: parentID,
			Permission: model.ResourcePermissionRead,
		},
	})
	require.ErrorIs(t, err, domain.ErrTransferNotFound)
}

func TestGetTransferRequiresAuthorization(t *testing.T) {
	parentID := uuid.New()
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: uuid.New(), WorkspaceID: uuid.New(), NamespaceID: uuid.New(),
		TargetParentID: &parentID, TargetName: "payload.bin", CreatedBy: uuid.New(),
		Protocol: model.TransferProtocolDirect, ExpectedSize: 1,
		RequestHash: strings64("c"), IdempotencyKey: "key-3",
		TemporaryObjectKey: "transfers/x/content",
		ExpiresAt: time.Now().UTC().Add(time.Hour), Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	get := NewGetTransferHandler(getTransferRepoStub{session: session})
	_, err = get.Handle(context.Background(), GetTransfer{
		WorkspaceID: session.WorkspaceID(), SessionID: session.ID(),
		Authorization: command.Authorization{Reason: "deny"},
	})
	require.True(t, errors.Is(err, domain.ErrResourceDenied))
}

func strings64(value string) string {
	out := ""
	for range 64 {
		out += value
	}
	return out
}
