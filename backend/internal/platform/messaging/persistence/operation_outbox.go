package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

type OperationRecord struct {
	ID             string
	EventID        string
	Subscriber     string
	OperationType  string
	IdempotencyKey string
	Payload        json.RawMessage
	Attempts       int
	NextAttemptAt  time.Time
	LockedAt       *time.Time
	LockedBy       string
	CompletedAt    *time.Time
	LastError      string
	CreatedAt      time.Time
}

type OperationStore interface {
	Insert(ctx context.Context, operation OperationRecord) error
	Claim(ctx context.Context, owner string, limit int, now time.Time, lease time.Duration) ([]OperationRecord, error)
	MarkCompleted(ctx context.Context, id, owner string, at time.Time) error
	MarkFailed(ctx context.Context, id, owner, reason string, at time.Time) error
	Reschedule(ctx context.Context, id, owner, reason string, next time.Time) error
}

type operationStore struct {
	db *gorm.DB
}

func NewOperationStore(db *gorm.DB) OperationStore {
	return &operationStore{db: db}
}

func (s *operationStore) Insert(ctx context.Context, operation OperationRecord) error {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return err
	}
	return tx.Create(&OperationOutbox{
		ID:             operation.ID,
		EventID:        operation.EventID,
		Subscriber:     operation.Subscriber,
		OperationType:  operation.OperationType,
		IdempotencyKey: operation.IdempotencyKey,
		Payload:        operation.Payload,
		NextAttemptAt:  operation.NextAttemptAt,
		CreatedAt:      operation.CreatedAt,
	}).Error
}

func (s *operationStore) Claim(
	ctx context.Context,
	owner string,
	limit int,
	now time.Time,
	lease time.Duration,
) ([]OperationRecord, error) {
	if limit <= 0 {
		return []OperationRecord{}, nil
	}

	var rows []OperationOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH candidates AS (
				SELECT id
				FROM platform.operation_outbox
				WHERE (status = 'pending' AND next_attempt_at <= @now)
				   OR (status = 'executing' AND locked_at <= @lease_expired_at)
				ORDER BY created_at, id
				FOR UPDATE SKIP LOCKED
				LIMIT @limit
			)
			UPDATE platform.operation_outbox AS operation
			SET status = 'executing',
			    locked_at = @now,
			    locked_by = @owner,
			    attempts = operation.attempts + 1
			FROM candidates
			WHERE operation.id = candidates.id
			RETURNING operation.*
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

	records := make([]OperationRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, operationRecord(row))
	}
	return records, nil
}

func (s *operationStore) MarkCompleted(ctx context.Context, id, owner string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&OperationOutbox{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, "executing", owner).
		Updates(map[string]any{
			"status":       "completed",
			"completed_at": at,
			"locked_at":    nil,
			"locked_by":    "",
			"last_error":   "",
		})
	return requireLeaseUpdate(result)
}

func (s *operationStore) MarkFailed(ctx context.Context, id, owner, reason string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&OperationOutbox{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, "executing", owner).
		Updates(map[string]any{
			"status":       "failed",
			"completed_at": at,
			"locked_at":    nil,
			"locked_by":    "",
			"last_error":   reason,
		})
	return requireLeaseUpdate(result)
}

func (s *operationStore) Reschedule(ctx context.Context, id, owner, reason string, next time.Time) error {
	result := s.db.WithContext(ctx).Model(&OperationOutbox{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, "executing", owner).
		Updates(map[string]any{
			"status":          "pending",
			"next_attempt_at": next,
			"locked_at":       nil,
			"locked_by":       "",
			"completed_at":    nil,
			"last_error":      reason,
		})
	return requireLeaseUpdate(result)
}

func operationClaimable(status string, nextAttemptAt time.Time, lockedAt *time.Time, now time.Time, lease time.Duration) bool {
	switch status {
	case "pending":
		return !nextAttemptAt.After(now)
	case "executing":
		return lockedAt != nil && !lockedAt.After(now.Add(-lease))
	default:
		return false
	}
}

func operationRecord(row OperationOutbox) OperationRecord {
	return OperationRecord{
		ID:             row.ID,
		EventID:        row.EventID,
		Subscriber:     row.Subscriber,
		OperationType:  row.OperationType,
		IdempotencyKey: row.IdempotencyKey,
		Payload:        row.Payload,
		Attempts:       row.Attempts,
		NextAttemptAt:  row.NextAttemptAt,
		LockedAt:       row.LockedAt,
		LockedBy:       row.LockedBy,
		CompletedAt:    row.CompletedAt,
		LastError:      row.LastError,
		CreatedAt:      row.CreatedAt,
	}
}
