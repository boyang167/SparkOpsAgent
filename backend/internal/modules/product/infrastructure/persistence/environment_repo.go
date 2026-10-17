package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type environmentModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	ProductID   string
	WorkspaceID string
	Name        string
	EnvType     string
	ClusterID   string
	Namespace   string
	Variables   string `gorm:"type:jsonb"`
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (environmentModel) TableName() string { return "product.environments" }

type environmentProjectModel struct {
	EnvironmentID          string `gorm:"column:environment_id;primaryKey"`
	ProjectID              string `gorm:"column:project_id;primaryKey"`
	Enabled                bool
	CurrentImageTag        string
	CurrentDeploymentRunID *string
	LastDeployedAt         *time.Time
}

func (environmentProjectModel) TableName() string { return "product.environment_projects" }

type EnvironmentRepository struct{ db *gorm.DB }

func NewEnvironmentRepository(db *gorm.DB) repository.EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

func environmentAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&environmentModel{}, &environmentProjectModel{})
}

func (r *EnvironmentRepository) SaveEnvironment(ctx context.Context, env *model.Environment) error {
	row, err := toEnvironmentModel(env)
	if err != nil {
		return err
	}
	return postgres.DB(ctx, r.db).Save(row).Error
}

func (r *EnvironmentRepository) FindEnvironmentByID(ctx context.Context, id kernel.EnvironmentID) (*model.Environment, error) {
	var row environmentModel
	err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEnvironmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromEnvironmentModel(row)
}

func (r *EnvironmentRepository) ListEnvironmentsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.Environment, error) {
	var rows []environmentModel
	if err := postgres.DB(ctx, r.db).Where("product_id = ?", productID.String()).
		Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Environment, 0, len(rows))
	for _, row := range rows {
		env, err := fromEnvironmentModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	return out, nil
}

func (r *EnvironmentRepository) DeleteEnvironment(ctx context.Context, id kernel.EnvironmentID) error {
	result := postgres.DB(ctx, r.db).Where("id = ?", id.String()).Delete(&environmentModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrEnvironmentNotFound
	}
	return nil
}

func (r *EnvironmentRepository) CountEnvironmentsByProduct(ctx context.Context, productID kernel.ProductID) (int, error) {
	var count int64
	err := postgres.DB(ctx, r.db).Model(&environmentModel{}).
		Where("product_id = ?", productID.String()).Count(&count).Error
	return int(count), err
}

func (r *EnvironmentRepository) SaveEnvironmentProject(ctx context.Context, row *model.EnvironmentProject) error {
	m := environmentProjectModel{
		EnvironmentID: row.EnvironmentID.String(), ProjectID: row.ProjectID.String(),
		Enabled: row.Enabled, CurrentImageTag: row.CurrentImageTag,
		LastDeployedAt: row.LastDeployedAt,
	}
	if row.CurrentDeploymentRunID != nil {
		v := row.CurrentDeploymentRunID.String()
		m.CurrentDeploymentRunID = &v
	}
	return postgres.DB(ctx, r.db).Save(&m).Error
}

func (r *EnvironmentRepository) ListEnvironmentProjects(ctx context.Context, envID kernel.EnvironmentID) ([]*model.EnvironmentProject, error) {
	var rows []environmentProjectModel
	if err := postgres.DB(ctx, r.db).Where("environment_id = ?", envID.String()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.EnvironmentProject, 0, len(rows))
	for _, row := range rows {
		item, err := fromEnvironmentProjectModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *EnvironmentRepository) EnsureEnvironmentProjects(ctx context.Context, envID kernel.EnvironmentID, projectIDs []kernel.ProjectID) error {
	for _, pid := range projectIDs {
		var count int64
		if err := postgres.DB(ctx, r.db).Model(&environmentProjectModel{}).
			Where("environment_id = ? AND project_id = ?", envID.String(), pid.String()).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := postgres.DB(ctx, r.db).Create(&environmentProjectModel{
			EnvironmentID: envID.String(), ProjectID: pid.String(), Enabled: true,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *EnvironmentRepository) FindEnvironmentProject(ctx context.Context, envID kernel.EnvironmentID, projectID kernel.ProjectID) (*model.EnvironmentProject, error) {
	var row environmentProjectModel
	err := postgres.DB(ctx, r.db).
		Where("environment_id = ? AND project_id = ?", envID.String(), projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEnvironmentProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromEnvironmentProjectModel(row)
}

func toEnvironmentModel(env *model.Environment) (*environmentModel, error) {
	vars, err := json.Marshal(env.Variables)
	if err != nil {
		return nil, err
	}
	if len(vars) == 0 {
		vars = []byte("{}")
	}
	return &environmentModel{
		ID: env.ID.String(), ProductID: env.ProductID.String(), WorkspaceID: env.WorkspaceID.String(),
		Name: env.Name, EnvType: string(env.EnvType), ClusterID: env.ClusterID.String(),
		Namespace: env.Namespace, Variables: string(vars), Status: string(env.Status),
		CreatedBy: env.CreatedBy.String(), CreatedAt: env.CreatedAt, UpdatedAt: env.UpdatedAt,
	}, nil
}

func fromEnvironmentModel(row environmentModel) (*model.Environment, error) {
	id, err := kernel.ParseEnvironmentID(row.ID)
	if err != nil {
		return nil, err
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, err
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, err
	}
	clusterID, err := kernel.ParseClusterInfoID(row.ClusterID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	vars := map[string]string{}
	if row.Variables != "" {
		_ = json.Unmarshal([]byte(row.Variables), &vars)
	}
	return &model.Environment{
		ID: id, ProductID: productID, WorkspaceID: wsID, Name: row.Name,
		EnvType: model.EnvironmentType(row.EnvType), ClusterID: clusterID, Namespace: row.Namespace,
		Variables: vars, Status: model.EnvironmentStatus(row.Status), CreatedBy: createdBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func fromEnvironmentProjectModel(row environmentProjectModel) (*model.EnvironmentProject, error) {
	envID, err := kernel.ParseEnvironmentID(row.EnvironmentID)
	if err != nil {
		return nil, err
	}
	projectID, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, err
	}
	var runID *kernel.DeploymentRunID
	if row.CurrentDeploymentRunID != nil && *row.CurrentDeploymentRunID != "" {
		parsed, err := kernel.ParseDeploymentRunID(*row.CurrentDeploymentRunID)
		if err != nil {
			return nil, err
		}
		runID = &parsed
	}
	return &model.EnvironmentProject{
		EnvironmentID: envID, ProjectID: projectID, Enabled: row.Enabled,
		CurrentImageTag: row.CurrentImageTag, CurrentDeploymentRunID: runID,
		LastDeployedAt: row.LastDeployedAt,
	}, nil
}

func productProjectIDs(ctx context.Context, db *gorm.DB, productID kernel.ProductID) ([]kernel.ProjectID, error) {
	type row struct{ ID string }
	var rows []row
	if err := db.WithContext(ctx).Table("workspace_app.projects").
		Select("id").Where("product_id = ? AND deleted_at IS NULL", productID.String()).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]kernel.ProjectID, 0, len(rows))
	for _, item := range rows {
		pid, err := kernel.ParseProjectID(item.ID)
		if err != nil {
			continue
		}
		out = append(out, pid)
	}
	return out, nil
}

func ClusterAuthorized(ctx context.Context, db *gorm.DB, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("platform_admin.workspace_clusters").
		Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
		Count(&count).Error
	return count > 0, err
}

func projectProductID(ctx context.Context, db *gorm.DB, projectID kernel.ProjectID) (kernel.ProductID, error) {
	var raw *string
	if err := db.WithContext(ctx).Table("workspace_app.projects").
		Select("product_id").Where("id = ?", projectID.String()).Scan(&raw).Error; err != nil {
		return kernel.ProductID{}, err
	}
	if raw == nil || *raw == "" {
		return kernel.ProductID{UUID: uuid.Nil}, nil
	}
	return kernel.ParseProductID(*raw)
}
