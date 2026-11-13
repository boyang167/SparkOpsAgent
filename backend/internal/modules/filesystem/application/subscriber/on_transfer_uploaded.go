package subscriber

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const TransferVerifyCommitSubscriber = "filesystem.transfer.verify-commit"

type transferSessionReader interface {
	GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error)
}

type transferVerifier interface {
	Handle(context.Context, command.VerifyTransfer) error
}

type transferCommitter interface {
	Handle(context.Context, command.CommitTransfer) (command.CommitTransferResult, error)
}

type OnTransferUploaded struct {
	verify   transferVerifier
	commit   transferCommitter
	sessions transferSessionReader
}

func NewOnTransferUploaded(
	verify transferVerifier,
	commit transferCommitter,
	sessions transferSessionReader,
) *OnTransferUploaded {
	return &OnTransferUploaded{verify: verify, commit: commit, sessions: sessions}
}

func (s *OnTransferUploaded) Handle(ctx context.Context, event events.Event) error {
	uploaded, ok := event.(filesystemevent.TransferUploaded)
	if !ok {
		return nil
	}
	if s.verify == nil || s.commit == nil {
		return errors.New("transfer verify-commit handlers are required")
	}
	if err := s.verify.Handle(ctx, command.VerifyTransfer{
		SessionID: uploaded.SessionID, WorkspaceID: uploaded.WorkspaceID,
	}); err != nil {
		if errors.Is(err, model.ErrTransferIntegrity) || errors.Is(err, domain.ErrTransferStatusConflict) {
			return messaging.Permanent(err)
		}
		return err
	}
	if s.sessions == nil {
		return errors.New("transfer session reader is required")
	}
	session, err := s.sessions.GetTransferByID(ctx, uploaded.SessionID)
	if err != nil {
		return err
	}
	switch session.Status() {
	case model.TransferRejected, model.TransferFailed, model.TransferAborted, model.TransferExpired, model.TransferCompleted:
		return nil
	case model.TransferScanning:
		// proceed to commit
	default:
		return messaging.Permanent(domain.ErrTransferStatusConflict)
	}
	if _, err := s.commit.Handle(ctx, command.CommitTransfer{
		SessionID: uploaded.SessionID, WorkspaceID: uploaded.WorkspaceID,
	}); err != nil {
		if errors.Is(err, domain.ErrTransferStatusConflict) {
			return messaging.Permanent(err)
		}
		return err
	}
	return nil
}
