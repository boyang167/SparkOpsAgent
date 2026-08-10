package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type NotificationView struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Link      string  `json:"link"`
	Read      bool    `json:"read"`
	CreatedAt string  `json:"created_at"`
	ReadAt    *string `json:"read_at,omitempty"`
}

type ListNotificationsResult struct {
	Items       []NotificationView `json:"items"`
	UnreadCount int                `json:"unread_count"`
}

type ListNotificationsHandler struct {
	repo repository.NotificationRepository
}

func NewListNotificationsHandler(repo repository.NotificationRepository) *ListNotificationsHandler {
	return &ListNotificationsHandler{repo: repo}
}

func (h *ListNotificationsHandler) Handle(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	limit int,
) (*ListNotificationsResult, error) {
	items, err := h.repo.ListByMember(ctx, memberID, wsID, limit)
	if err != nil {
		return nil, err
	}
	unread, err := h.repo.CountUnread(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]NotificationView, 0, len(items))
	for _, n := range items {
		out = append(out, toView(n))
	}
	return &ListNotificationsResult{Items: out, UnreadCount: unread}, nil
}

func toView(n *model.Notification) NotificationView {
	view := NotificationView{
		ID: n.ID.String(), Type: string(n.Type), Title: n.Title,
		Body: n.Body, Link: n.Link, Read: n.IsRead(),
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
	if n.ReadAt != nil {
		s := n.ReadAt.Format(time.RFC3339)
		view.ReadAt = &s
	}
	return view
}
