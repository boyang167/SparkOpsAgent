package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type transferLister interface {
	ListTransfers(
		context.Context,
		uuid.UUID,
		*model.TransferStatus,
		int,
		string,
	) ([]*model.TransferSession, string, error)
}

type ListTransfers struct {
	WorkspaceID uuid.UUID
	Status      model.TransferStatus
	Limit       int
	Cursor      string
}

type ListTransfersHandler struct {
	repo transferLister
}

func NewListTransfersHandler(repo transferLister) *ListTransfersHandler {
	if repo == nil {
		panic("filesystem list transfers requires repository")
	}
	return &ListTransfersHandler{repo: repo}
}

func (h *ListTransfersHandler) Handle(
	ctx context.Context,
	q ListTransfers,
) ([]*model.TransferSession, string, error) {
	if q.WorkspaceID == uuid.Nil {
		return nil, "", model.ErrInvalidTransfer
	}
	var status *model.TransferStatus
	if q.Status != "" {
		if !validListTransferStatus(q.Status) {
			return nil, "", model.ErrInvalidTransfer
		}
		value := q.Status
		status = &value
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	rows, next, err := h.repo.ListTransfers(ctx, q.WorkspaceID, status, limit, q.Cursor)
	if err != nil {
		return nil, "", err
	}
	for _, row := range rows {
		if row == nil || row.WorkspaceID() != q.WorkspaceID {
			return nil, "", model.ErrInvalidTransfer
		}
	}
	return rows, next, nil
}

func validListTransferStatus(status model.TransferStatus) bool {
	switch status {
	case model.TransferInitiated, model.TransferUploading, model.TransferPaused,
		model.TransferVerifying, model.TransferScanning, model.TransferCommitting,
		model.TransferCompleted, model.TransferFailed, model.TransferRejected,
		model.TransferAborted, model.TransferExpired:
		return true
	default:
		return false
	}
}
