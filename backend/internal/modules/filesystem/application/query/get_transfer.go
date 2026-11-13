package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type transferReader interface {
	GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error)
	ListTransferParts(context.Context, uuid.UUID) ([]model.TransferPart, error)
}

type GetTransfer struct {
	WorkspaceID uuid.UUID
	SessionID   uuid.UUID
	Authorization command.Authorization
}

type GetTransferResult struct {
	Session    *model.TransferSession
	Parts      []model.TransferPart
	Descriptor command.TransferDescriptor
}

type GetTransferHandler struct {
	repo transferReader
}

func NewGetTransferHandler(repo transferReader) *GetTransferHandler {
	if repo == nil {
		panic("filesystem get transfer requires repository")
	}
	return &GetTransferHandler{repo: repo}
}

func (h *GetTransferHandler) Handle(ctx context.Context, q GetTransfer) (GetTransferResult, error) {
	var out GetTransferResult
	if q.SessionID == uuid.Nil || q.WorkspaceID == uuid.Nil {
		return out, model.ErrInvalidTransfer
	}
	session, err := h.repo.GetTransferByID(ctx, q.SessionID)
	if err != nil {
		return out, err
	}
	if session.WorkspaceID() != q.WorkspaceID {
		return out, domain.ErrTransferNotFound
	}
	parent := session.TargetParentID()
	if parent == nil {
		return out, domain.ErrParentNotFound
	}
	if authErr := command.RequireResourceAllowed(q.Authorization, q.WorkspaceID, *parent, model.ResourcePermissionRead); authErr != nil {
		return out, authErr
	}
	parts, err := h.repo.ListTransferParts(ctx, q.SessionID)
	if err != nil {
		return out, err
	}
	return GetTransferResult{
		Session: session, Parts: parts, Descriptor: command.DescriptorFromSession(session),
	}, nil
}
