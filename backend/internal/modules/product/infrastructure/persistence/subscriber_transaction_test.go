package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProductSubscriberRepositoriesUseContextTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE product.product_requirements (
			id TEXT PRIMARY KEY, product_id TEXT, product_plan_id TEXT, project_id TEXT,
			project_name TEXT, project_tag TEXT, build_id TEXT, description TEXT,
			status TEXT, multicluster_status TEXT, operator TEXT, other_info TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE product.environment_projects (
			environment_id TEXT, project_id TEXT, enabled BOOLEAN,
			current_image_tag TEXT, current_deployment_run_id TEXT, last_deployed_at DATETIME,
			PRIMARY KEY(environment_id, project_id)
		)
	`).Error)

	productRepo := NewProductRepository(db)
	environmentRepo := NewEnvironmentRepository(db)
	buildID := kernel.NewBuildID()
	requirementID := kernel.NewProductRequirementID()
	productID := kernel.NewProductID()
	planID := kernel.NewProductPlanID()
	projectID := kernel.NewProjectID()
	environmentID := kernel.NewEnvironmentID()
	rollback := errors.New("force rollback")

	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		tx := postgres.DB(ctx, db)
		require.NoError(t, tx.Exec(`
			INSERT INTO product.product_requirements
				(id, product_id, product_plan_id, project_id, build_id, status, other_info, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, requirementID.String(), productID.String(), planID.String(), projectID.String(), buildID.String(),
			string(model.ProductRequirementStatusRunning)).Error)

		requirement, findErr := productRepo.FindRequirementByBuildID(ctx, buildID)
		require.NoError(t, findErr)
		requirement.Status = model.ProductRequirementStatusCompleted
		require.NoError(t, productRepo.UpdateRequirement(ctx, requirement))

		require.NoError(t, environmentRepo.SaveEnvironmentProject(ctx, &model.EnvironmentProject{
			EnvironmentID:   environmentID,
			ProjectID:       projectID,
			Enabled:         true,
			CurrentImageTag: "image:v1",
		}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	var requirementCount, environmentCount int64
	require.NoError(t, db.Table("product.product_requirements").Count(&requirementCount).Error)
	require.NoError(t, db.Table("product.environment_projects").Count(&environmentCount).Error)
	require.Zero(t, requirementCount)
	require.Zero(t, environmentCount)
}
