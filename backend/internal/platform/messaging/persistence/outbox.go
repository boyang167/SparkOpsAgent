package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"gorm.io/gorm"
)

type OutboxRecord struct {
	events.Envelope
	Attempts      int
	NextAttemptAt time.Time
	LockedAt      *time.Time
	LockedBy      string
	CreatedAt     time.Time
}

type OutboxStore interface {
	Insert(ctx context.Context, envelope events.Envelope) error
	Claim(ctx context.Context, owner string, limit int, now time.Time, lease time.Duration) ([]OutboxRecord, error)
	MarkPublished(ctx context.Context, id, owner string, at time.Time) error
	Reschedule(ctx context.Context, id, owner, reason string, next time.Time) error
	Stats(ctx context.Context, now time.Time) (OutboxStats, error)
}

type OutboxStats struct {
	Pending   int64
	OldestAge time.Duration
}

type outboxStore struct {
	db *gorm.DB
}

func NewOutboxStore(db *gorm.DB) OutboxStore {
	return &outboxStore{db: db}
}

func (s *outboxStore) Insert(ctx context.Context, envelope events.Envelope) error {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return err
	}
	return tx.Create(&EventOutbox{
		ID:               envelope.ID,
		EventType:        envelope.Type,
		EventVersion:     envelope.Version,
		AggregateType:    envelope.AggregateType,
		AggregateID:      envelope.AggregateID,
		AggregateVersion: envelope.AggregateVersion,
		Payload:          json.RawMessage(envelope.Payload),
		OccurredAt:       envelope.OccurredAt,
		CorrelationID:    optionalString(envelope.CorrelationID),
		CausationID:      optionalString(envelope.CausationID),
	}).Error
}

func (s *outboxStore) Claim(
	ctx context.Context,
	owner string,
	limit int,
	now time.Time,
	lease time.Duration,
) ([]OutboxRecord, error) {
	if limit <= 0 {
		return []OutboxRecord{}, nil
	}

	var rows []EventOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH candidates AS (
				SELECT id
				FROM platform.event_outbox
				WHERE (status = 'pending' AND next_attempt_at <= @now)
				   OR (status = 'publishing' AND locked_at <= @lease_expired_at)
				ORDER BY created_at, id
				FOR UPDATE SKIP LOCKED
				LIMIT @limit
			)
			UPDATE platform.event_outbox AS outbox
			SET status = 'publishing',
			    locked_at = @now,
			    locked_by = @owner,
			    attempts = outbox.attempts + 1
			FROM candidates
			WHERE outbox.id = candidates.id
			RETURNING outbox.*
		`,
			sql.Named("now", now),
			sql.Named("lease_expired_at", now.Add(-lease)),
			sql.Named("owner", owner),
			sql.Named("limit", limit),
		).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}

	records := make([]OutboxRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, outboxRecord(row))
	}
	return records, nil
}

func (s *outboxStore) MarkPublished(ctx context.Context, id, owner string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&EventOutbox{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, "publishing", owner).
		Updates(map[string]any{
			"status":       "published",
			"published_at": at,
			"locked_at":    nil,
			"locked_by":    "",
			"last_error":   "",
		})
	return requireLeaseUpdate(result)
}

func (s *outboxStore) Reschedule(ctx context.Context, id, owner, reason string, next time.Time) error {
	result := s.db.WithContext(ctx).Model(&EventOutbox{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, "publishing", owner).
		Updates(map[string]any{
			"status":          "pending",
			"next_attempt_at": next,
			"locked_at":       nil,
			"locked_by":       "",
			"last_error":      reason,
		})
	return requireLeaseUpdate(result)
}

func (s *outboxStore) Stats(ctx context.Context, now time.Time) (OutboxStats, error) {
	query := s.db.WithContext(ctx).Model(&EventOutbox{}).
		Where("status IN ?", []string{"pending", "publishing"})
	var stats OutboxStats
	if err := query.Count(&stats.Pending).Error; err != nil {
		return OutboxStats{}, err
	}
	if stats.Pending == 0 {
		return stats, nil
	}
	var oldest EventOutbox
	if err := query.Order("created_at ASC").Select("created_at").Take(&oldest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return stats, nil
		}
		return OutboxStats{}, err
	}
	stats.OldestAge = now.Sub(oldest.CreatedAt)
	if stats.OldestAge < 0 {
		stats.OldestAge = 0
	}
	return stats, nil
}

func outboxClaimable(status string, nextAttemptAt time.Time, lockedAt *time.Time, now time.Time, lease time.Duration) bool {
	switch status {
	case "pending":
		return !nextAttemptAt.After(now)
	case "publishing":
		return lockedAt != nil && !lockedAt.After(now.Add(-lease))
	default:
		return false
	}
}

func outboxRecord(row EventOutbox) OutboxRecord {
	return OutboxRecord{
		Envelope: events.Envelope{
			ID:               row.ID,
			Type:             row.EventType,
			Version:          row.EventVersion,
			AggregateType:    row.AggregateType,
			AggregateID:      row.AggregateID,
			AggregateVersion: row.AggregateVersion,
			OccurredAt:       row.OccurredAt,
			CorrelationID:    stringValue(row.CorrelationID),
			CausationID:      stringValue(row.CausationID),
			Payload:          row.Payload,
		},
		Attempts:      row.Attempts,
		NextAttemptAt: row.NextAttemptAt,
		LockedAt:      row.LockedAt,
		LockedBy:      row.LockedBy,
		CreatedAt:     row.CreatedAt,
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
