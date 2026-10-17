package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type txRow struct {
	ID string `gorm:"primaryKey"`
}

func TestTransactorRollsBackAllWrites(t *testing.T) {
	db := openTransactionSQLite(t)
	txr := postgres.NewTransactor(db)
	errSentinel := errors.New("stop")

	err := txr.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, postgres.DB(ctx, db).Create(&txRow{ID: "one"}).Error)
		return errSentinel
	})

	require.ErrorIs(t, err, errSentinel)
	var count int64
	require.NoError(t, db.Model(&txRow{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestRequireTxRejectsContextWithoutTransaction(t *testing.T) {
	_, err := postgres.RequireTx(context.Background())
	require.ErrorContains(t, err, "transaction required")
}

func TestDirectTransactorCallsFunctionWithOriginalContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "value")

	err := postgres.NewDirectTransactor().WithinTransaction(ctx, func(got context.Context) error {
		require.Same(t, ctx, got)
		return nil
	})

	require.NoError(t, err)
}

func openTransactionSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&txRow{}))
	return db
}
