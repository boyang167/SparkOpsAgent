package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

func TestVerifyTransferRejectsIntegrityMismatch(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	providerID := uuid.New()
	content := []byte("payload")
	cmd := transferFixture(ns)
	cmd.Size = int64(len(content))
	cmd.SHA256 = storage.SHA256([]byte("different"))
	cmd.ProviderID = providerID
	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: session.TemporaryObjectKey(),
		Body:      bytes.NewReader(content),
		Size:      int64(len(content)),
		SHA256:    storage.SHA256(content),
	})
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	verify := NewVerifyTransferHandler(repo, postgres.NewTransactor(db), MapProviders{providerID: provider}, port.NoopScanner{})
	err = verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID})
	require.ErrorIs(t, err, model.ErrTransferIntegrity)
}

func TestVerifyTransferRejectsInfectedFile(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	providerID := uuid.New()
	content := []byte("infected")
	checksum := storage.SHA256(content)
	cmd := transferFixture(ns)
	cmd.Size = int64(len(content))
	cmd.SHA256 = checksum
	cmd.ProviderID = providerID
	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	_, err = provider.Put(ctx, storage.PutRequest{ObjectKey: session.TemporaryObjectKey(), Body: bytes.NewReader(content), Size: int64(len(content)), SHA256: checksum})
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	verify := NewVerifyTransferHandler(repo, postgres.NewTransactor(db), MapProviders{providerID: provider}, port.NoopScanner{
		Result: port.ScanResult{Clean: false, ThreatName: "EICAR", Engine: "noop"},
	})
	err = verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID})
	require.NoError(t, err)
	updated, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferRejected, updated.Status())
}

func TestVerifyTransferRestrictedScanFailureRejected(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	providerID := uuid.New()
	content := []byte("restricted")
	checksum := storage.SHA256(content)
	cmd := transferFixture(ns)
	cmd.Size = int64(len(content))
	cmd.SHA256 = checksum
	cmd.ProviderID = providerID
	cmd.Classification = model.TransferClassificationRestricted
	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	_, err = provider.Put(ctx, storage.PutRequest{ObjectKey: session.TemporaryObjectKey(), Body: bytes.NewReader(content), Size: int64(len(content)), SHA256: checksum})
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	verify := NewVerifyTransferHandler(repo, postgres.NewTransactor(db), MapProviders{providerID: provider}, port.NoopScanner{Err: port.ErrScannerUnavailable})
	err = verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID})
	require.NoError(t, err)
	updated, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferRejected, updated.Status())
}

func TestVerifyTransferMarksMissingObjectFailed(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	providerID := uuid.New()
	cmd := transferFixture(ns)
	cmd.ProviderID = providerID
	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	verify := NewVerifyTransferHandler(repo, postgres.NewTransactor(db), MapProviders{providerID: provider}, port.NoopScanner{})
	require.NoError(t, verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID, WorkspaceID: ns.WorkspaceID}))
	updated, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferFailed, updated.Status())
	require.Equal(t, "object_missing", updated.ErrorCode())
}

func TestVerifyTransferScannerUnavailableFailsRetryably(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	providerID := uuid.New()
	content := []byte("internal")
	checksum := storage.SHA256(content)
	cmd := transferFixture(ns)
	cmd.Size = int64(len(content))
	cmd.SHA256 = checksum
	cmd.ProviderID = providerID
	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	_, err = provider.Put(ctx, storage.PutRequest{ObjectKey: session.TemporaryObjectKey(), Body: bytes.NewReader(content), Size: int64(len(content)), SHA256: checksum})
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	verify := NewVerifyTransferHandler(repo, postgres.NewTransactor(db), MapProviders{providerID: provider}, port.NoopScanner{Err: port.ErrScannerUnavailable})
	err = verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID})
	require.Error(t, err)
	require.False(t, errors.Is(err, domain.ErrTransferStatusConflict))
	updated, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferFailed, updated.Status())
}
