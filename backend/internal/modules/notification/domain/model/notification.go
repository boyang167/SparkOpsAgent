package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type NotificationType string

const (
	NotificationTypeRunCompleted         NotificationType = "run.completed"
	NotificationTypeWorkflowRunCompleted NotificationType = "workflow.run.completed"
	NotificationTypeBuildCompleted       NotificationType = "build.completed"
	NotificationTypeProjectReviewPending NotificationType = "project.review.pending"
)

type Notification struct {
	ID          kernel.NotificationID
	MemberID    kernel.MemberID
	WorkspaceID kernel.WorkspaceID
	Type        NotificationType
	Title       string
	Body        string
	Link        string
	ReadAt      *time.Time
	CreatedAt   time.Time
}

func NewNotification(
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	typ NotificationType,
	title, body, link string,
) (*Notification, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, domain.ErrInvalidTitle
	}
	return &Notification{
		ID:          kernel.NewNotificationID(),
		MemberID:    memberID,
		WorkspaceID: wsID,
		Type:        typ,
		Title:       title,
		Body:        strings.TrimSpace(body),
		Link:        strings.TrimSpace(link),
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (n *Notification) MarkRead() {
	if n.ReadAt != nil {
		return
	}
	now := time.Now().UTC()
	n.ReadAt = &now
}

func (n *Notification) IsRead() bool { return n.ReadAt != nil }
