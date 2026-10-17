package persistence

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

type InboxStore interface {
	TryInsert(ctx context.Context, eventID, subscriber, eventType string, now time.Time) (bool, error)
}

type inboxStore struct {
	db *gorm.DB
}

func NewInboxStore(db *gorm.DB) InboxStore {
	return &inboxStore{db: db}
}

func (s *inboxStore) TryInsert(
	ctx context.Context,
	eventID string,
	subscriber string,
	eventType string,
	now time.Time,
) (bool, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return false, err
	}
	result := tx.Exec(`
		INSERT INTO platform.event_inbox(event_id, subscriber, event_type, processed_at, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (event_id, subscriber) DO NOTHING
	`, eventID, subscriber, eventType, now, now)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
