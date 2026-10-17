package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrOperationIdempotencyConflict = errors.New("operation idempotency key conflicts with existing intent")

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
	Stats(ctx context.Context, now time.Time) (OperationStats, error)
}

type OperationStats struct {
	Pending   int64
	OldestAge time.Duration
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
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(&OperationOutbox{
		ID:             operation.ID,
		EventID:        operation.EventID,
		Subscriber:     operation.Subscriber,
		OperationType:  operation.OperationType,
		IdempotencyKey: operation.IdempotencyKey,
		Payload:        operation.Payload,
		NextAttemptAt:  operation.NextAttemptAt,
		CreatedAt:      operation.CreatedAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var existing OperationOutbox
	if err := tx.Where(
		"idempotency_key = ?",
		operation.IdempotencyKey,
	).Take(&existing).Error; err != nil {
		return fmt.Errorf("read duplicate operation intent: %w", err)
	}
	if operationEventIdentityEqual(
		operation.IdempotencyKey,
		existing.EventID,
		operation.EventID,
	) &&
		existing.Subscriber == operation.Subscriber &&
		existing.OperationType == operation.OperationType &&
		jsonPayloadEqual(existing.Payload, operation.Payload) {
		return nil
	}
	return fmt.Errorf(
		"%w: key %q",
		ErrOperationIdempotencyConflict,
		operation.IdempotencyKey,
	)
}

func operationEventIdentityEqual(key, leftEventID, rightEventID string) bool {
	stableID, _, _ := strings.Cut(key, ":")
	leftIsEvent := leftEventID == stableID
	rightIsEvent := rightEventID == stableID
	if leftIsEvent != rightIsEvent {
		return false
	}
	return !leftIsEvent || leftEventID == rightEventID
}

func jsonPayloadEqual(left, right json.RawMessage) bool {
	leftValue, leftErr := decodeExactJSON(left)
	rightValue, rightErr := decodeExactJSON(right)
	if leftErr == nil && rightErr == nil {
		return exactJSONEqual(leftValue, rightValue)
	}
	return bytes.Equal(left, right)
}

func decodeExactJSON(payload json.RawMessage) (any, error) {
	if !json.Valid(payload) {
		return nil, fmt.Errorf("invalid JSON payload")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func exactJSONEqual(left, right any) bool {
	switch leftValue := left.(type) {
	case nil:
		return right == nil
	case bool:
		rightValue, ok := right.(bool)
		return ok && leftValue == rightValue
	case string:
		rightValue, ok := right.(string)
		return ok && leftValue == rightValue
	case json.Number:
		rightValue, ok := right.(json.Number)
		if !ok {
			return false
		}
		leftNumber, leftOK := new(big.Rat).SetString(leftValue.String())
		rightNumber, rightOK := new(big.Rat).SetString(rightValue.String())
		return leftOK && rightOK && leftNumber.Cmp(rightNumber) == 0
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for index := range leftValue {
			if !exactJSONEqual(leftValue[index], rightValue[index]) {
				return false
			}
		}
		return true
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, value := range leftValue {
			rightItem, exists := rightValue[key]
			if !exists || !exactJSONEqual(value, rightItem) {
				return false
			}
		}
		return true
	default:
		return false
	}
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

func (s *operationStore) Stats(ctx context.Context, now time.Time) (OperationStats, error) {
	query := s.db.WithContext(ctx).Model(&OperationOutbox{}).
		Where("status IN ?", []string{"pending", "executing"})
	var stats OperationStats
	if err := query.Count(&stats.Pending).Error; err != nil {
		return OperationStats{}, err
	}
	if stats.Pending == 0 {
		return stats, nil
	}
	var oldest OperationOutbox
	if err := query.Order("created_at ASC").Select("created_at").Take(&oldest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return stats, nil
		}
		return OperationStats{}, err
	}
	stats.OldestAge = now.Sub(oldest.CreatedAt)
	if stats.OldestAge < 0 {
		stats.OldestAge = 0
	}
	return stats, nil
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
