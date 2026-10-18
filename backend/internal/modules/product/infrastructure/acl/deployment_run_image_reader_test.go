package acl

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeploymentRunImageReaderFindByRunIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.deployment_runs (
		id TEXT PRIMARY KEY,
		image_repository TEXT NOT NULL DEFAULT '',
		image_tag TEXT NOT NULL DEFAULT ''
	)`).Error)
	runID := kernel.NewDeploymentRunID()
	require.NoError(t, db.Exec(
		`INSERT INTO delivery.deployment_runs (id, image_repository, image_tag) VALUES (?, ?, ?)`,
		runID.String(), "harbor.example/team/app", "v2.1.0",
	).Error)

	reader := NewDeploymentRunImageReader(db)
	got, err := reader.FindByRunIDs(context.Background(), []kernel.DeploymentRunID{runID})
	require.NoError(t, err)
	require.Equal(t, "harbor.example/team/app", got[runID.String()].Repository)
	require.Equal(t, "v2.1.0", got[runID.String()].Tag)
}
