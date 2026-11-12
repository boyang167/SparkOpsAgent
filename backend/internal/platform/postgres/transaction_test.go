package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
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

func TestGormTransactorBeginFailure(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	beginErr := errors.New("begin failed")
	mock.ExpectBegin().WillReturnError(beginErr)

	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(context.Context) error {
		t.Fatal("callback must not run")
		return nil
	})

	require.ErrorIs(t, err, beginErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormTransactorCallbackErrorRollsBack(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	callbackErr := errors.New("callback failed")
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(context.Context) error {
		return callbackErr
	})

	require.ErrorIs(t, err, callbackErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormTransactorJoinsRollbackError(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(rollbackErr)

	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(context.Context) error {
		return callbackErr
	})

	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, rollbackErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormTransactorPanicRollsBackAndRepanics(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	panicValue := "panic fixture"

	require.PanicsWithValue(t, panicValue, func() {
		_ = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(context.Context) error {
			panic(panicValue)
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormTransactorNestedCallDoesNotBeginTwice(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	transactor := postgres.NewTransactor(db)
	nestedCalls := 0

	err := transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return transactor.WithinTransaction(ctx, func(context.Context) error {
			nestedCalls++
			return nil
		})
	})

	require.NoError(t, err)
	require.Equal(t, 1, nestedCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGormTransactorCommitErrorHasUnknownOutcomeSentinel(t *testing.T) {
	db, mock := openTransactionSQLMock(t)
	commitErr := errors.New("commit failed")
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(commitErr)

	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, postgres.ErrCommitOutcomeUnknown)
	require.NoError(t, mock.ExpectationsWereMet())
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

func openTransactionSQLMock(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db, mock
}
