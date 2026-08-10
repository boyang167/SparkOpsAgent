package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateNotificationHandler struct {
	repo repository.NotificationRepository
}

func NewCreateNotificationHandler(repo repository.NotificationRepository) *CreateNotificationHandler {
	return &CreateNotificationHandler{repo: repo}
}

func (h *CreateNotificationHandler) Handle(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	typ model.NotificationType,
	title, body, link string,
) (*model.Notification, error) {
	n, err := model.NewNotification(memberID, wsID, typ, title, body, link)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

type MarkReadHandler struct {
	repo repository.NotificationRepository
}

func NewMarkReadHandler(repo repository.NotificationRepository) *MarkReadHandler {
	return &MarkReadHandler{repo: repo}
}

func (h *MarkReadHandler) Handle(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID, id kernel.NotificationID) error {
	n, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if n.MemberID != memberID || n.WorkspaceID != wsID {
		return domain.ErrNotificationNotFound
	}
	n.MarkRead()
	return h.repo.Update(ctx, n)
}

type MarkAllReadHandler struct {
	repo repository.NotificationRepository
}

func NewMarkAllReadHandler(repo repository.NotificationRepository) *MarkAllReadHandler {
	return &MarkAllReadHandler{repo: repo}
}

func (h *MarkAllReadHandler) Handle(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (int, error) {
	return h.repo.MarkAllRead(ctx, memberID, wsID)
}
