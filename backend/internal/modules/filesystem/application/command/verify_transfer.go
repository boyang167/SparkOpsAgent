package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type VerifyTransfer struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
}

type VerifyTransferHandler struct {
	repo      repository.FilesystemRepository
	tx        postgres.Transactor
	providers StorageProviderCatalog
	scanner   port.Scanner
}

func NewVerifyTransferHandler(
	repo repository.FilesystemRepository,
	tx postgres.Transactor,
	providers StorageProviderCatalog,
	scanner port.Scanner,
) *VerifyTransferHandler {
	requireMutationDependencies(repo, tx)
	if providers == nil {
		panic("filesystem verify transfer requires storage providers")
	}
	if scanner == nil {
		panic("filesystem verify transfer requires scanner")
	}
	return &VerifyTransferHandler{repo: repo, tx: tx, providers: providers, scanner: scanner}
}

func (h *VerifyTransferHandler) Handle(ctx context.Context, cmd VerifyTransfer) error {
	if cmd.SessionID == uuid.Nil {
		return model.ErrInvalidTransfer
	}
	session, err := h.prepareForScanning(ctx, cmd)
	if err != nil {
		if errors.Is(err, errVerifyTerminal) {
			return nil
		}
		return err
	}
	providerID := session.ProviderID()
	if providerID == nil {
		return model.ErrInvalidTransfer
	}
	scanResult, scanErr := h.scanner.Scan(ctx, port.ObjectRef{ProviderID: *providerID, ObjectKey: session.TemporaryObjectKey()})
	if scanErr != nil {
		if err := h.markScanFailure(ctx, cmd, scanErr, false); err != nil {
			return errors.Join(scanErr, err)
		}
		updated, readErr := h.repo.GetTransferByID(ctx, cmd.SessionID)
		if readErr == nil && updated.Status() == model.TransferRejected {
			return nil
		}
		return scanErr
	}
	if !scanResult.Clean {
		if err := h.markScanFailure(ctx, cmd, port.ErrScannerInfected, true); err != nil {
			return err
		}
		return nil
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, lockErr := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if lockErr != nil {
			return lockErr
		}
		if session.Status() != model.TransferScanning {
			return domain.ErrTransferStatusConflict
		}
		if session.ScanResult() != nil {
			return nil
		}
		now := time.Now().UTC()
		result := model.TransferScanResult{
			Clean: true, ThreatName: scanResult.ThreatName, Engine: scanResult.Engine,
			Signature: scanResult.Signature, CompletedAt: now,
		}
		version := session.Version()
		if err := session.SetScanResult(result, version, now); err != nil {
			return err
		}
		return h.saveTransfer(txCtx, session, version, model.TransferScanning, model.TransferScanning, &result)
	})
}

func (h *VerifyTransferHandler) prepareForScanning(ctx context.Context, cmd VerifyTransfer) (*model.TransferSession, error) {
	var prepared *model.TransferSession
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if err != nil {
			return err
		}
		if cmd.WorkspaceID != uuid.Nil && session.WorkspaceID() != cmd.WorkspaceID {
			return domain.ErrNamespaceMismatch
		}
		switch session.Status() {
		case model.TransferCompleted, model.TransferRejected, model.TransferFailed,
			model.TransferAborted, model.TransferExpired:
			prepared = session
			return errVerifyTerminal
		case model.TransferScanning:
			if session.ScanResult() != nil {
				prepared = session
				return errVerifyTerminal
			}
		case model.TransferUploading, model.TransferVerifying:
		default:
			return domain.ErrTransferStatusConflict
		}
		if session.Status() == model.TransferScanning {
			prepared = session
			return nil
		}
		providerID := session.ProviderID()
		if providerID == nil {
			return model.ErrInvalidTransfer
		}
		provider, err := h.providers.Provider(txCtx, *providerID)
		if err != nil {
			return err
		}
		info, statErr := provider.Stat(txCtx, session.TemporaryObjectKey())
		if statErr != nil {
			if errors.Is(statErr, storage.ErrNotFound) {
				return errTransferObjectMissingPersist
			}
			return statErr
		}
		checksum := strings.ToLower(strings.TrimSpace(info.SHA256))
		if info.Size != session.ExpectedSize() || !validSHA256(checksum) {
			return errTransferIntegrityPersist
		}
		if expected := strings.ToLower(strings.TrimSpace(session.ExpectedSHA256())); expected != "" && !strings.EqualFold(checksum, expected) {
			return errTransferIntegrityPersist
		}
		now := time.Now().UTC()
		if session.Status() == model.TransferUploading {
			version := session.Version()
			if err := session.StartVerify(version, now); err != nil {
				return err
			}
			if err := h.saveTransfer(txCtx, session, version, model.TransferUploading, model.TransferVerifying, nil); err != nil {
				return err
			}
			session, err = h.repo.LockTransfer(txCtx, cmd.SessionID)
			if err != nil {
				return err
			}
		}
		version := session.Version()
		if err := session.SetActual(info.Size, checksum, version, now); err != nil {
			return errTransferIntegrityPersist
		}
		if err := h.saveTransfer(txCtx, session, version, model.TransferVerifying, model.TransferVerifying, nil); err != nil {
			return err
		}
		session, err = h.repo.LockTransfer(txCtx, cmd.SessionID)
		if err != nil {
			return err
		}
		version = session.Version()
		if err := session.StartScan(version, now); err != nil {
			return err
		}
		if err := h.saveTransfer(txCtx, session, version, model.TransferVerifying, model.TransferScanning, nil); err != nil {
			return err
		}
		session, err = h.repo.LockTransfer(txCtx, cmd.SessionID)
		if err != nil {
			return err
		}
		prepared = session
		return nil
	})
	if errors.Is(err, errVerifyTerminal) {
		return prepared, nil
	}
	if errors.Is(err, errTransferIntegrityPersist) {
		if failErr := h.markTransferFailed(ctx, cmd.SessionID, "integrity_mismatch"); failErr != nil {
			return nil, failErr
		}
		if failed, readErr := h.repo.GetTransferByID(ctx, cmd.SessionID); readErr == nil &&
			failed.Status() == model.TransferFailed {
			return failed, errVerifyTerminal
		}
		return nil, model.ErrTransferIntegrity
	}
	if errors.Is(err, errTransferObjectMissingPersist) {
		if failErr := h.markTransferFailed(ctx, cmd.SessionID, "object_missing"); failErr != nil {
			return nil, failErr
		}
		if failed, readErr := h.repo.GetTransferByID(ctx, cmd.SessionID); readErr == nil &&
			failed.Status() == model.TransferFailed {
			return failed, errVerifyTerminal
		}
		return nil, storage.ErrNotFound
	}
	return prepared, err
}

var (
	errVerifyTerminal               = errors.New("filesystem verify terminal")
	errTransferIntegrityPersist     = errors.New("filesystem verify integrity persist")
	errTransferObjectMissingPersist = errors.New("filesystem verify object missing persist")
)

func (h *VerifyTransferHandler) markScanFailure(ctx context.Context, cmd VerifyTransfer, cause error, reject bool) error {
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if err != nil {
			return err
		}
		if reject || session.Classification() == model.TransferClassificationRestricted {
			return h.persistReject(txCtx, session, rejectReason(cause))
		}
		return h.persistFailure(txCtx, session, "scanner_unavailable")
	})
}

func rejectReason(cause error) string {
	if errors.Is(cause, port.ErrScannerInfected) {
		return "malware_detected"
	}
	return "scan_unavailable"
}

func (h *VerifyTransferHandler) markTransferFailed(ctx context.Context, sessionID uuid.UUID, code string) error {
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := h.repo.LockTransfer(txCtx, sessionID)
		if err != nil {
			return err
		}
		if session.Status() == model.TransferFailed || session.Status() == model.TransferRejected {
			return nil
		}
		return h.persistFailure(txCtx, session, code)
	})
}

func (h *VerifyTransferHandler) persistFailure(ctx context.Context, session *model.TransferSession, code string) error {
	from := session.Status()
	version := session.Version()
	if err := session.Fail(code, version, time.Now().UTC()); err != nil {
		return err
	}
	patch := repository.TransferStatePatch{
		StatusFrom: from,
		StatusTo:   model.TransferFailed,
		ErrorCode:  &code,
	}
	return h.repo.SaveTransferStateCAS(ctx, session.WorkspaceID(), session.ID(), version, patch)
}

func (h *VerifyTransferHandler) persistReject(ctx context.Context, session *model.TransferSession, code string) error {
	version := session.Version()
	if session.Status() != model.TransferScanning {
		now := time.Now().UTC()
		if session.Status() == model.TransferUploading || session.Status() == model.TransferVerifying {
			if err := session.StartVerify(version, now); err != nil {
				return err
			}
			if err := h.saveTransfer(ctx, session, version, session.Status(), model.TransferVerifying, nil); err != nil {
				return err
			}
			session, err := h.repo.LockTransfer(ctx, session.ID())
			if err != nil {
				return err
			}
			version = session.Version()
			if err := session.StartScan(version, now); err != nil {
				return err
			}
			if err := h.saveTransfer(ctx, session, version, model.TransferVerifying, model.TransferScanning, nil); err != nil {
				return err
			}
			session, err = h.repo.LockTransfer(ctx, session.ID())
			if err != nil {
				return err
			}
			version = session.Version()
		}
	}
	if err := session.Reject(code, version, time.Now().UTC()); err != nil {
		return err
	}
	patch := repository.TransferStatePatch{
		StatusFrom: model.TransferScanning,
		StatusTo:   model.TransferRejected,
		ErrorCode:  &code,
	}
	return h.repo.SaveTransferStateCAS(ctx, session.WorkspaceID(), session.ID(), version, patch)
}

func (h *VerifyTransferHandler) saveTransfer(
	ctx context.Context,
	session *model.TransferSession,
	expectedVersion int64,
	from, to model.TransferStatus,
	scan *model.TransferScanResult,
) error {
	patch := repository.TransferStatePatch{StatusFrom: from, StatusTo: to, ScanResult: scan}
	if size, ok := session.ActualSize(); ok {
		patch.ActualSize = &size
	}
	if checksum := session.ActualSHA256(); checksum != "" {
		patch.ActualSHA256 = checksum
	}
	if providerID := session.ProviderID(); providerID != nil {
		patch.ProviderID = providerID
	}
	return h.repo.SaveTransferStateCAS(ctx, session.WorkspaceID(), session.ID(), expectedVersion, patch)
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
