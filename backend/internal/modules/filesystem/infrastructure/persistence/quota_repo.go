package persistence

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func (r *Repository) ReserveTransferQuota(ctx context.Context, workspaceID uuid.UUID, bytes, files int64) (uuid.UUID, error) {
	if workspaceID == uuid.Nil || bytes < 0 || files < 0 {
		return uuid.Nil, domain.ErrQuotaExceeded
	}
	db := postgres.DB(ctx, r.db)
	var account quotaAccountRow
	result := db.Raw(`
		UPDATE filesystem.quota_accounts
		SET reserved_bytes = reserved_bytes + ?,
		    reserved_files = reserved_files + ?,
		    version = version + 1,
		    updated_at = ?
		WHERE workspace_id = ?
		  AND used_bytes <= ? - reserved_bytes
		  AND ? <= ? - used_bytes - reserved_bytes
		  AND used_files <= ? - reserved_files
		  AND ? <= ? - used_files - reserved_files
		  AND (max_bytes IS NULL OR (
		        used_bytes <= max_bytes
		    AND reserved_bytes <= max_bytes - used_bytes
		    AND ? <= max_bytes - used_bytes - reserved_bytes))
		  AND (max_files IS NULL OR (
		        used_files <= max_files
		    AND reserved_files <= max_files - used_files
		    AND ? <= max_files - used_files - reserved_files))
		RETURNING *
	`, bytes, files, time.Now().UTC(), workspaceID,
		int64(math.MaxInt64), bytes, int64(math.MaxInt64),
		int64(math.MaxInt64), files, int64(math.MaxInt64),
		bytes, files,
	).Scan(&account)
	if result.Error != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
			return uuid.Nil, domain.ErrTransactionRetry
		}
		return uuid.Nil, wrapPersistence(result.Error)
	}
	if account.ID != uuid.Nil {
		return account.ID, nil
	}
	var count int64
	if err := db.Model(&quotaAccountRow{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return uuid.Nil, wrapPersistence(err)
	}
	if count == 0 {
		return uuid.Nil, domain.ErrNamespaceNotFound
	}
	return uuid.Nil, domain.ErrQuotaExceeded
}

func (r *Repository) CreateQuotaReservation(ctx context.Context, reservation repository.QuotaReservation) error {
	if reservation.ID == uuid.Nil || reservation.QuotaAccountID == uuid.Nil ||
		reservation.WorkspaceID == uuid.Nil || reservation.SessionID == uuid.Nil ||
		reservation.Bytes < 0 || reservation.Files < 0 || reservation.State != "RESERVED" ||
		reservation.ExpiresAt.IsZero() {
		return domain.ErrPersistence
	}
	sessionID := reservation.SessionID
	err := postgres.DB(ctx, r.db).Create(&quotaReservationRow{
		ID: reservation.ID, QuotaAccountID: reservation.QuotaAccountID,
		WorkspaceID: reservation.WorkspaceID, SessionID: &sessionID,
		Bytes: reservation.Bytes, Files: reservation.Files, State: reservation.State,
		ExpiresAt: reservation.ExpiresAt, CreatedAt: time.Now().UTC(),
	}).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrTransferConflict
	}
	if postgres.DB(ctx, r.db).Dialector.Name() == "sqlite" && isSQLiteBusy(err) {
		return domain.ErrTransactionRetry
	}
	return wrapPersistence(err)
}
