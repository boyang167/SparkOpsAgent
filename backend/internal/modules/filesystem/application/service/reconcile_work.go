package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type RepositoryWorkRunner struct {
	db   *gorm.DB
	repo repository.FilesystemRepository
}

func NewRepositoryWorkRunner(db *gorm.DB, repo repository.FilesystemRepository) *RepositoryWorkRunner {
	return &RepositoryWorkRunner{db: db, repo: repo}
}

var staleTransferStatuses = []model.TransferStatus{
	model.TransferInitiated,
	model.TransferUploading,
	model.TransferPaused,
	model.TransferVerifying,
	model.TransferScanning,
	model.TransferCommitting,
}

func (w *RepositoryWorkRunner) RunPass(ctx context.Context, cfg ReconcilerConfig, now time.Time) (ReconcileSummary, error) {
	if w == nil || w.db == nil || w.repo == nil {
		return ReconcileSummary{}, nil
	}
	cfg = cfg.withDefaults()
	summary := ReconcileSummary{}

	released, err := w.releaseExpiredReservations(ctx, now, cfg.BatchSize)
	if err != nil {
		return summary, err
	}
	summary.ExpiredReservationsReleased = released

	expiredTransfers, err := w.expireStaleTransfers(ctx, now, cfg.BatchSize)
	if err != nil {
		return summary, err
	}
	summary.StaleTransfersExpired = expiredTransfers

	issues, err := w.countBlobReplicaIssues(ctx)
	if err != nil {
		return summary, err
	}
	summary.BlobReplicaIssues = issues

	claimed, err := w.claimBacklog(ctx, cfg.BatchSize)
	if err != nil {
		return summary, err
	}
	summary.BacklogJobsClaimed = claimed

	return summary, nil
}

func (w *RepositoryWorkRunner) releaseExpiredReservations(ctx context.Context, now time.Time, limit int) (int, error) {
	type reservationRow struct {
		WorkspaceID uuid.UUID
		SessionID   *uuid.UUID
	}
	var rows []reservationRow
	err := postgres.DB(ctx, w.db).
		Table("filesystem.quota_reservations").
		Select("workspace_id", "session_id").
		Where("state = ? AND expires_at <= ?", "RESERVED", now).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}
	released := 0
	for _, row := range rows {
		if row.SessionID == nil || *row.SessionID == uuid.Nil {
			continue
		}
		sessionID := *row.SessionID
		if err := w.repo.ReleaseQuotaReservation(ctx, row.WorkspaceID, sessionID); err != nil {
			continue
		}
		released++
	}
	return released, nil
}

func (w *RepositoryWorkRunner) expireStaleTransfers(ctx context.Context, now time.Time, limit int) (int, error) {
	type transferRow struct {
		ID          uuid.UUID
		WorkspaceID uuid.UUID
		Status      model.TransferStatus
	}
	var rows []transferRow
	err := postgres.DB(ctx, w.db).
		Table("filesystem.transfer_sessions").
		Select("id", "workspace_id", "status").
		Where("expires_at <= ? AND status IN ?", now, staleTransferStatuses).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}
	expired := 0
	for _, row := range rows {
		result := postgres.DB(ctx, w.db).
			Table("filesystem.transfer_sessions").
			Where("id = ? AND status = ?", row.ID, row.Status).
			Updates(map[string]any{
				"status":     model.TransferExpired,
				"updated_at": now,
			})
		if result.Error != nil || result.RowsAffected == 0 {
			continue
		}
		_ = w.repo.ReleaseQuotaReservation(ctx, row.WorkspaceID, row.ID)
		expired++
	}
	return expired, nil
}

func (w *RepositoryWorkRunner) countBlobReplicaIssues(ctx context.Context) (int, error) {
	var pendingBlobs int64
	if err := postgres.DB(ctx, w.db).
		Table("filesystem.blobs").
		Where("state = ?", model.BlobStatePending).
		Count(&pendingBlobs).Error; err != nil {
		return 0, err
	}
	var replicaIssues int64
	if err := postgres.DB(ctx, w.db).
		Table("filesystem.storage_replicas").
		Where("state IN ?", []model.ReplicaState{
			model.ReplicaStateCreating,
			model.ReplicaStateVerifying,
			model.ReplicaStateFailed,
		}).
		Count(&replicaIssues).Error; err != nil {
		return 0, err
	}
	return int(pendingBlobs + replicaIssues), nil
}

func (w *RepositoryWorkRunner) claimBacklog(ctx context.Context, limit int) (int, error) {
	var blobIDs []uuid.UUID
	err := postgres.DB(ctx, w.db).
		Table("filesystem.blobs").
		Where("state IN ?", []model.BlobState{model.BlobStateGCPending, model.BlobStateTombstoned}).
		Limit(limit).
		Pluck("id", &blobIDs).Error
	if err != nil {
		return 0, err
	}
	return len(blobIDs), nil
}
