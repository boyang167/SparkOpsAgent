package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

)

var reconciliationPassCount atomic.Int32

type countingWorkRunner struct{}

func (countingWorkRunner) RunPass(context.Context, ReconcilerConfig, time.Time) (ReconcileSummary, error) {
	reconciliationPassCount.Add(1)
	return ReconcileSummary{}, nil
}

func TestOnlyLeaseOwnerReconciles(t *testing.T) {
	reconciliationPassCount.Store(0)
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	lease, err := NewDBLease(db)
	require.NoError(t, err)

	cfg := ReconcilerConfig{LeaseTTL: time.Minute, BatchSize: 100, GCGracePeriod: 7 * 24 * time.Hour}
	first := NewReconciler("reconciler-a", lease, countingWorkRunner{}, cfg)
	second := NewReconciler("reconciler-b", lease, countingWorkRunner{}, cfg)

	require.NoError(t, first.RunOnce(ctx))
	require.ErrorIs(t, second.RunOnce(ctx), ErrLeaseNotOwned)
	assert.Equal(t, 1, reconciliationPasses(t))
}

func twoReconcilers(t *testing.T) (*Reconciler, *Reconciler) {
	t.Helper()
	reconciliationPassCount.Store(0)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	lease, err := NewDBLease(db)
	require.NoError(t, err)

	cfg := ReconcilerConfig{LeaseTTL: time.Minute, BatchSize: 100, GCGracePeriod: 7 * 24 * time.Hour}
	first := NewReconciler("reconciler-a", lease, countingWorkRunner{}, cfg)
	second := NewReconciler("reconciler-b", lease, countingWorkRunner{}, cfg)
	return first, second
}

func reconciliationPasses(t *testing.T) int {
	t.Helper()
	return int(reconciliationPassCount.Load())
}
