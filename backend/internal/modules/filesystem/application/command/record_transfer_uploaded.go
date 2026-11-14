package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type RecordTransferUploaded struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
	Size        int64
	SHA256      string
}

type RecordTransferUploadedHandler struct {
	repo repository.FilesystemRepository
	tx   postgres.Transactor
	bus  messaging.Bus
}

func NewRecordTransferUploadedHandler(
	repo repository.FilesystemRepository,
	tx postgres.Transactor,
	bus messaging.Bus,
) *RecordTransferUploadedHandler {
	requireMutationDependencies(repo, tx)
	return &RecordTransferUploadedHandler{repo: repo, tx: tx, bus: bus}
}

func (h *RecordTransferUploadedHandler) Handle(ctx context.Context, cmd RecordTransferUploaded) error {
	if cmd.SessionID == uuid.Nil {
		return model.ErrInvalidTransfer
	}
	cmd.SHA256 = strings.ToLower(strings.TrimSpace(cmd.SHA256))
	var workspaceID uuid.UUID
	var namespaceID uuid.UUID
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if err != nil {
			return err
		}
		if cmd.WorkspaceID != uuid.Nil && session.WorkspaceID() != cmd.WorkspaceID {
			return domain.ErrNamespaceMismatch
		}
		workspaceID = session.WorkspaceID()
		namespaceID = session.NamespaceID()
		switch session.Status() {
		case model.TransferCompleted, model.TransferRejected, model.TransferFailed,
			model.TransferAborted, model.TransferExpired:
			return nil
		case model.TransferScanning, model.TransferVerifying, model.TransferCommitting:
			return nil
		}
		if session.Status() == model.TransferInitiated || session.Status() == model.TransferPaused {
			from := session.Status()
			version := session.Version()
			now := time.Now().UTC()
			if err := session.StartUpload(version, now); err != nil {
				return err
			}
			if err := h.repo.SaveTransferStateCAS(txCtx, workspaceID, session.ID(), version, repository.TransferStatePatch{
				StatusFrom: from,
				StatusTo:   model.TransferUploading,
			}); err != nil {
				return err
			}
		} else if session.Status() != model.TransferUploading {
			return domain.ErrTransferStatusConflict
		}
		if cmd.Size < 0 {
			return model.ErrInvalidTransfer
		}
		if cmd.SHA256 != "" && !validSHA256(cmd.SHA256) {
			return model.ErrInvalidTransferChecksum
		}
		_ = cmd.Size
		_ = cmd.SHA256
		return nil
	})
	if err != nil {
		return err
	}
	if h.bus == nil {
		return errors.New("filesystem record transfer uploaded requires event bus")
	}
	return h.bus.Publish(ctx, event.TransferUploaded{
		BaseEvent:   sharedevents.NewBaseEvent(event.TransferUploadedName),
		SessionID:   cmd.SessionID,
		WorkspaceID: workspaceID,
		NamespaceID: namespaceID,
	})
}
