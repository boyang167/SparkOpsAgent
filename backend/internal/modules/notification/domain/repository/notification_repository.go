package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type NotificationRepository interface {
	Save(ctx context.Context, n *model.Notification) error
	Update(ctx context.Context, n *model.Notification) error
	FindByID(ctx context.Context, id kernel.NotificationID) (*model.Notification, error)
	ListByMember(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID, limit int) ([]*model.Notification, error)
	CountUnread(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (int, error)
	MarkAllRead(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (int, error)
}
