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
	var account quotaAccountRow
	if err := postgres.DB(ctx, r.db).Where("workspace_id = ?", workspaceID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, domain.ErrNamespaceNotFound
		}
		return uuid.Nil, wrapPersistence(err)
	}
	return r.ReserveNamespaceQuota(ctx, account.NamespaceID, bytes, files)
}

func (r *Repository) ReserveNamespaceQuota(ctx context.Context, namespaceID uuid.UUID, bytes, files int64) (uuid.UUID, error) {
	if namespaceID == uuid.Nil || bytes < 0 || files < 0 {
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
		WHERE namespace_id = ?
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
	`, bytes, files, time.Now().UTC(), namespaceID,
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
	if err := db.Model(&quotaAccountRow{}).Where("namespace_id = ?", namespaceID).Count(&count).Error; err != nil {
		return uuid.Nil, wrapPersistence(err)
	}
	if count == 0 {
		return uuid.Nil, domain.ErrNamespaceNotFound
	}
	return uuid.Nil, domain.ErrQuotaExceeded
}

func (r *Repository) CommitQuotaReservation(ctx context.Context, workspaceID, sessionID uuid.UUID) error {
	if workspaceID == uuid.Nil || sessionID == uuid.Nil {
		return domain.ErrPersistence
	}
	db := postgres.DB(ctx, r.db)
	var reservation quotaReservationRow
	err := db.Where("workspace_id = ? AND session_id = ?", workspaceID, sessionID).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrPersistence
	}
	if err != nil {
		return wrapPersistence(err)
	}
	return r.commitQuotaReservation(ctx, reservation)
}

func (r *Repository) CommitNamespaceQuotaReservation(ctx context.Context, namespaceID, sessionID uuid.UUID) error {
	if namespaceID == uuid.Nil || sessionID == uuid.Nil {
		return domain.ErrPersistence
	}
	var reservation quotaReservationRow
	err := postgres.DB(ctx, r.db).Where("namespace_id = ? AND session_id = ?", namespaceID, sessionID).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrPersistence
	}
	if err != nil {
		return wrapPersistence(err)
	}
	return r.commitQuotaReservation(ctx, reservation)
}

func (r *Repository) commitQuotaReservation(ctx context.Context, reservation quotaReservationRow) error {
	db := postgres.DB(ctx, r.db)
	if reservation.State == "COMMITTED" {
		return nil
	}
	if reservation.State != "RESERVED" {
		return domain.ErrPersistence
	}
	result := db.Model(&quotaReservationRow{}).
		Where("id = ? AND state = ?", reservation.ID, "RESERVED").
		Updates(map[string]any{"state": "COMMITTED"})
	if result.Error != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
			return domain.ErrTransactionRetry
		}
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		var current quotaReservationRow
		if err := db.Where("id = ?", reservation.ID).First(&current).Error; err != nil {
			return wrapPersistence(err)
		}
		if current.State == "COMMITTED" {
			return nil
		}
		return domain.ErrPersistence
	}
	accountResult := db.Model(&quotaAccountRow{}).
		Where("namespace_id = ? AND reserved_bytes >= ? AND reserved_files >= ?", reservation.NamespaceID, reservation.Bytes, reservation.Files).
		Updates(map[string]any{
			"reserved_bytes": gorm.Expr("reserved_bytes - ?", reservation.Bytes),
			"reserved_files": gorm.Expr("reserved_files - ?", reservation.Files),
			"used_bytes":     gorm.Expr("used_bytes + ?", reservation.Bytes),
			"used_files":     gorm.Expr("used_files + ?", reservation.Files),
			"version":        gorm.Expr("version + 1"),
			"updated_at":     time.Now().UTC(),
		})
	if accountResult.Error != nil {
		return wrapPersistence(accountResult.Error)
	}
	if accountResult.RowsAffected == 0 {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (r *Repository) ReleaseQuotaReservation(ctx context.Context, workspaceID, sessionID uuid.UUID) error {
	if workspaceID == uuid.Nil || sessionID == uuid.Nil {
		return domain.ErrPersistence
	}
	db := postgres.DB(ctx, r.db)
	var reservation quotaReservationRow
	err := db.Where("workspace_id = ? AND session_id = ?", workspaceID, sessionID).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return wrapPersistence(err)
	}
	return r.releaseQuotaReservation(ctx, reservation)
}

func (r *Repository) ReleaseNamespaceQuotaReservation(ctx context.Context, namespaceID, sessionID uuid.UUID) error {
	if namespaceID == uuid.Nil || sessionID == uuid.Nil {
		return domain.ErrPersistence
	}
	var reservation quotaReservationRow
	err := postgres.DB(ctx, r.db).Where("namespace_id = ? AND session_id = ?", namespaceID, sessionID).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return wrapPersistence(err)
	}
	return r.releaseQuotaReservation(ctx, reservation)
}

func (r *Repository) releaseQuotaReservation(ctx context.Context, reservation quotaReservationRow) error {
	db := postgres.DB(ctx, r.db)
	if reservation.State == "RELEASED" {
		return nil
	}
	if reservation.State == "COMMITTED" {
		return domain.ErrPersistence
	}
	if reservation.State != "RESERVED" {
		return domain.ErrPersistence
	}
	result := db.Model(&quotaReservationRow{}).
		Where("id = ? AND state = ?", reservation.ID, "RESERVED").
		Updates(map[string]any{"state": "RELEASED"})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		var current quotaReservationRow
		if err := db.Where("id = ?", reservation.ID).First(&current).Error; err != nil {
			return wrapPersistence(err)
		}
		if current.State == "RELEASED" {
			return nil
		}
		return domain.ErrPersistence
	}
	accountResult := db.Model(&quotaAccountRow{}).
		Where("namespace_id = ? AND reserved_bytes >= ? AND reserved_files >= ?", reservation.NamespaceID, reservation.Bytes, reservation.Files).
		Updates(map[string]any{
			"reserved_bytes": gorm.Expr("reserved_bytes - ?", reservation.Bytes),
			"reserved_files": gorm.Expr("reserved_files - ?", reservation.Files),
			"version":        gorm.Expr("version + 1"),
			"updated_at":     time.Now().UTC(),
		})
	if accountResult.Error != nil {
		return wrapPersistence(accountResult.Error)
	}
	if accountResult.RowsAffected == 0 {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (r *Repository) CreateQuotaReservation(ctx context.Context, reservation repository.QuotaReservation) error {
	if reservation.ID == uuid.Nil || reservation.QuotaAccountID == uuid.Nil ||
		reservation.SessionID == uuid.Nil ||
		reservation.Bytes < 0 || reservation.Files < 0 || reservation.State != "RESERVED" ||
		reservation.ExpiresAt.IsZero() {
		return domain.ErrPersistence
	}
	if reservation.NamespaceID == uuid.Nil {
		var account quotaAccountRow
		if err := postgres.DB(ctx, r.db).Where("id = ?", reservation.QuotaAccountID).First(&account).Error; err != nil {
			return wrapPersistence(err)
		}
		reservation.NamespaceID = account.NamespaceID
	}
	sessionID := reservation.SessionID
	err := postgres.DB(ctx, r.db).Create(&quotaReservationRow{
		ID: reservation.ID, QuotaAccountID: reservation.QuotaAccountID,
		NamespaceID: reservation.NamespaceID, WorkspaceID: reservation.WorkspaceID, SessionID: &sessionID,
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
