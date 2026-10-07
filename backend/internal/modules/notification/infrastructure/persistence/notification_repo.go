package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS notification").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&notificationModel{})
}

type notificationModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	MemberID    string
	WorkspaceID string
	Type        string
	Title       string
	Body        string
	Link        string
	ReadAt      *time.Time
	postgres.AuditColumns
}

func (notificationModel) TableName() string { return "notification.notifications" }

type NotificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) repository.NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Save(ctx context.Context, n *model.Notification) error {
	row := toRow(n)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: n.CreatedAt, UpdatedAt: n.CreatedAt}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *NotificationRepository) Update(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Model(&notificationModel{}).Where("id = ?", n.ID.String()).Updates(map[string]any{
		"read_at":    n.ReadAt,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *NotificationRepository) FindByID(ctx context.Context, id kernel.NotificationID) (*model.Notification, error) {
	var row notificationModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotificationNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToModel(row)
}

func (r *NotificationRepository) ListByMember(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	limit int,
) ([]*model.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []notificationModel
	err := r.db.WithContext(ctx).
		Where("member_id = ? AND workspace_id = ?", memberID.String(), wsID.String()).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Notification, 0, len(rows))
	for _, row := range rows {
		n, err := rowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&notificationModel{}).
		Where("member_id = ? AND workspace_id = ? AND read_at IS NULL", memberID.String(), wsID.String()).
		Count(&count).Error
	return int(count), err
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (int, error) {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&notificationModel{}).
		Where("member_id = ? AND workspace_id = ? AND read_at IS NULL", memberID.String(), wsID.String()).
		Updates(map[string]any{"read_at": now, "updated_at": now})
	return int(result.RowsAffected), result.Error
}

func toRow(n *model.Notification) notificationModel {
	return notificationModel{
		ID: n.ID.String(), MemberID: n.MemberID.String(), WorkspaceID: n.WorkspaceID.String(),
		Type: string(n.Type), Title: n.Title, Body: n.Body, Link: n.Link, ReadAt: n.ReadAt,
	}
}

func rowToModel(row notificationModel) (*model.Notification, error) {
	id, _ := kernel.ParseNotificationID(row.ID)
	memberID, _ := kernel.ParseMemberID(row.MemberID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.Notification{
		ID: id, MemberID: memberID, WorkspaceID: wsID,
		Type: model.NotificationType(row.Type), Title: row.Title,
		Body: row.Body, Link: row.Link, ReadAt: row.ReadAt, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.NotificationRepository = (*NotificationRepository)(nil)
