package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type DBLease struct {
	db    *gorm.DB
	table string
}

type reconcilerLeaseRow struct {
	Name           string    `gorm:"column:name;primaryKey"`
	LeaseOwner     string    `gorm:"column:lease_owner;not null"`
	LeaseExpiresAt time.Time `gorm:"column:lease_expires_at;not null"`
	LastPassAt     *time.Time
	LastSummary    string `gorm:"column:last_summary"`
	UpdatedAt      time.Time
}

func leaseTableName(db *gorm.DB) string {
	if db != nil && db.Dialector.Name() == "sqlite" {
		return "reconciler_leases"
	}
	return "filesystem.reconciler_leases"
}

func NewDBLease(db *gorm.DB) (*DBLease, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	lease := &DBLease{db: db, table: leaseTableName(db)}
	if err := lease.ensureSchema(); err != nil {
		return nil, err
	}
	return lease, nil
}

func (l *DBLease) ensureSchema() error {
	switch l.db.Dialector.Name() {
	case "sqlite":
		return l.db.Exec(`CREATE TABLE IF NOT EXISTS reconciler_leases (
			name TEXT PRIMARY KEY,
			lease_owner TEXT NOT NULL,
			lease_expires_at DATETIME NOT NULL,
			last_pass_at DATETIME,
			last_summary TEXT,
			updated_at DATETIME NOT NULL
		)`).Error
	case "postgres":
		if err := l.db.Exec("CREATE SCHEMA IF NOT EXISTS filesystem").Error; err != nil {
			return err
		}
		return l.db.Exec(`CREATE TABLE IF NOT EXISTS filesystem.reconciler_leases (
			name TEXT PRIMARY KEY,
			lease_owner TEXT NOT NULL,
			lease_expires_at TIMESTAMPTZ NOT NULL,
			last_pass_at TIMESTAMPTZ,
			last_summary TEXT,
			updated_at TIMESTAMPTZ NOT NULL
		)`).Error
	default:
		return l.db.Table(l.table).AutoMigrate(&reconcilerLeaseRow{})
	}
}

func (l *DBLease) TryAcquireOrRenew(ctx context.Context, name, owner string, ttl time.Duration, now time.Time) (bool, error) {
	name = strings.TrimSpace(name)
	owner = strings.TrimSpace(owner)
	if name == "" || owner == "" || ttl <= 0 {
		return false, fmt.Errorf("invalid reconciler lease request")
	}
	expiresAt := now.Add(ttl)
	db := postgres.DB(ctx, l.db)
	var acquired bool
	err := db.Transaction(func(tx *gorm.DB) error {
		table := tx.Table(l.table)
		var row reconcilerLeaseRow
		err := table.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", name).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			createErr := tx.Table(l.table).Create(&reconcilerLeaseRow{
				Name: name, LeaseOwner: owner, LeaseExpiresAt: expiresAt, UpdatedAt: now,
			}).Error
			if createErr == nil {
				acquired = true
				return nil
			}
			if refetchErr := tx.Table(l.table).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("name = ?", name).First(&row).Error; refetchErr != nil {
				return createErr
			}
		} else if err != nil {
			return err
		}
		if row.LeaseOwner == owner || !row.LeaseExpiresAt.After(now) {
			result := tx.Table(l.table).
				Where("name = ? AND (lease_owner = ? OR lease_expires_at <= ?)", name, owner, now).
				Updates(map[string]any{
					"lease_owner": owner, "lease_expires_at": expiresAt, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			acquired = result.RowsAffected == 1
			return nil
		}
		acquired = false
		return nil
	})
	return acquired, err
}

func (l *DBLease) RecordPass(ctx context.Context, name, owner string, summary ReconcileSummary, now time.Time) error {
	summaryJSON, err := MarshalSummary(summary)
	if err != nil {
		return err
	}
	result := postgres.DB(ctx, l.db).Table(l.table).
		Where("name = ? AND lease_owner = ?", name, owner).
		Updates(map[string]any{
			"last_pass_at": now,
			"last_summary": summaryJSON,
			"updated_at":   now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("reconciler lease lost while recording summary")
	}
	return nil
}
