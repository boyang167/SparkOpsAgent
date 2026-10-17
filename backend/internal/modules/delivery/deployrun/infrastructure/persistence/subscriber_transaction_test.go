package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDeploymentRunRepositoryUsesContextTransaction(t *testing.T) {
	repo, db := newRunRepo(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	rollback := errors.New("force rollback")

	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, repo.Create(ctx, newRun(kernel.NewProjectID())))
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	var count int64
	require.NoError(t, db.Table("delivery.deployment_runs").Count(&count).Error)
	require.Zero(t, count, "deployment run must roll back with the Inbox transaction")
}
