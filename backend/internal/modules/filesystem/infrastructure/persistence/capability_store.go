package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type capabilityStore struct{ db *gorm.DB }

func NewCapabilityRepository(db *gorm.DB) repository.CapabilityRepository {
	return &capabilityStore{db: db}
}

func (s *capabilityStore) SaveCapability(ctx context.Context, record model.CapabilityRecord) error {
	row, err := capabilityFromRecord(record)
	if err != nil {
		return err
	}
	if err := postgres.DB(ctx, s.db).Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrIdempotencyConflict
		}
		return wrapPersistence(err)
	}
	return nil
}

func (s *capabilityStore) GetCapability(ctx context.Context, id uuid.UUID) (model.CapabilityRecord, error) {
	var row capabilityRow
	err := postgres.DB(ctx, s.db).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
	}
	if err != nil {
		return model.CapabilityRecord{}, wrapPersistence(err)
	}
	return row.record()
}

func (s *capabilityStore) GetCapabilityByHash(ctx context.Context, hash string) (model.CapabilityRecord, error) {
	var row capabilityRow
	err := postgres.DB(ctx, s.db).Where("token_hash = ?", hash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CapabilityRecord{}, domain.ErrCapabilityNotFound
	}
	if err != nil {
		return model.CapabilityRecord{}, wrapPersistence(err)
	}
	return row.record()
}

func (s *capabilityStore) RevokeCapability(ctx context.Context, id uuid.UUID, at time.Time) error {
	record, err := s.GetCapability(ctx, id)
	if err != nil {
		return err
	}
	if record.RevokedAt != nil {
		return nil
	}
	result := postgres.DB(ctx, s.db).Model(&capabilityRow{}).Where("id = ? AND revoked_at IS NULL", id).Update("revoked_at", at.UTC())
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrCapabilityNotFound
	}
	return nil
}

func (s *capabilityStore) SaveExportTask(ctx context.Context, task model.ExportTask) error {
	now := time.Now().UTC()
	row := exportTaskRow{
		ID: task.ID, SourceOrganizationID: task.SourceOrganizationID, TargetOrganizationID: task.TargetOrganizationID,
		SourceWorkspaceID: task.SourceWorkspaceID, TargetWorkspaceID: task.TargetWorkspaceID, SourceNodeID: task.SourceNodeID,
		TargetNodeID: task.TargetNodeID, Status: task.Status, ActorID: task.ActorID, CreatedAt: now, UpdatedAt: now,
	}
	if err := postgres.DB(ctx, s.db).Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrIdempotencyConflict
		}
		return wrapPersistence(err)
	}
	return nil
}

func (s *capabilityStore) GetExportTask(ctx context.Context, id uuid.UUID) (model.ExportTask, error) {
	var row exportTaskRow
	err := postgres.DB(ctx, s.db).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ExportTask{}, domain.ErrExportNotFound
	}
	if err != nil {
		return model.ExportTask{}, wrapPersistence(err)
	}
	return row.task(), nil
}

func (s *capabilityStore) MarkExportImported(ctx context.Context, id, targetNodeID uuid.UUID) error {
	return postgres.DB(ctx, s.db).Transaction(func(tx *gorm.DB) error {
		var row exportTaskRow
		err := tx.Where("id = ?", id).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrExportNotFound
		}
		if err != nil {
			return wrapPersistence(err)
		}
		if row.TargetNodeID != nil {
			if *row.TargetNodeID == targetNodeID {
				return nil
			}
			return domain.ErrIdempotencyConflict
		}
		result := tx.Model(&exportTaskRow{}).Where("id = ? AND target_node_id IS NULL", id).Updates(map[string]any{
			"target_node_id": targetNodeID, "status": model.ExportStatusImported, "updated_at": time.Now().UTC(),
		})
		if result.Error != nil {
			return wrapPersistence(result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrIdempotencyConflict
		}
		return nil
	})
}

func capabilityFromRecord(record model.CapabilityRecord) (capabilityRow, error) {
	resources, err := json.Marshal(record.ResourceIDs)
	if err != nil {
		return capabilityRow{}, domain.ErrCapabilityInvalid
	}
	actions, err := json.Marshal(record.Actions)
	if err != nil {
		return capabilityRow{}, domain.ErrCapabilityInvalid
	}
	return capabilityRow{
		ID: record.ID, TokenHash: record.TokenHash, OrganizationID: record.OrganizationID, WorkspaceID: record.WorkspaceID,
		ActorID: record.ActorID, AgentID: record.AgentID, RunID: record.RunID, ResourceIDs: string(resources), Actions: string(actions),
		Audience: record.Audience, ExpiresAt: record.ExpiresAt.UTC(), RevokedAt: record.RevokedAt, CreatedAt: time.Now().UTC(),
	}, nil
}

func (row capabilityRow) record() (model.CapabilityRecord, error) {
	var resources []uuid.UUID
	var actions []string
	if err := json.Unmarshal([]byte(row.ResourceIDs), &resources); err != nil {
		return model.CapabilityRecord{}, domain.ErrCapabilityInvalid
	}
	if err := json.Unmarshal([]byte(row.Actions), &actions); err != nil {
		return model.CapabilityRecord{}, domain.ErrCapabilityInvalid
	}
	return model.CapabilityRecord{
		ID: row.ID, TokenHash: row.TokenHash, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID,
		ActorID: row.ActorID, AgentID: row.AgentID, RunID: row.RunID, ResourceIDs: resources, Actions: actions,
		Audience: row.Audience, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
	}, nil
}

func (row exportTaskRow) task() model.ExportTask {
	return model.ExportTask{
		ID: row.ID, SourceOrganizationID: row.SourceOrganizationID, TargetOrganizationID: row.TargetOrganizationID,
		SourceWorkspaceID: row.SourceWorkspaceID, TargetWorkspaceID: row.TargetWorkspaceID, SourceNodeID: row.SourceNodeID,
		TargetNodeID: row.TargetNodeID, Status: row.Status, ActorID: row.ActorID,
	}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}
