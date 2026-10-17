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

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS product").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&productModel{}, &productPlanModel{}, &productRequirementModel{}); err != nil {
		return err
	}
	if err := releasePlanAutoMigrate(db); err != nil {
		return err
	}
	if err := sprintAutoMigrate(db); err != nil {
		return err
	}
	return environmentAutoMigrate(db)
}

type productModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	DisplayName string
	Description string `gorm:"type:text"`
	Version     string
	NextVersion string
	CreatedBy   string
	postgres.AuditColumns
}

func (productModel) TableName() string { return "product.products" }

type productPlanModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	ProductID   string
	Name        string
	Version     string
	Status      string
	Description string `gorm:"type:text"`
	Submitted   bool
	OtherInfo   string `gorm:"type:jsonb"`
	postgres.AuditColumns
}

func (productPlanModel) TableName() string { return "product.product_plans" }

type productRequirementModel struct {
	ID                 string `gorm:"column:id;primaryKey"`
	ProductID          string
	ProductPlanID      string
	ProjectID          string
	ProjectName        string
	ProjectTag         string
	BuildID            string
	Description        string `gorm:"type:text"`
	Status             string
	MulticlusterStatus string
	Operator           string
	OtherInfo          string `gorm:"type:jsonb"`
	postgres.AuditColumns
}

func (productRequirementModel) TableName() string { return "product.product_requirements" }

type ProductRepository struct{ db *gorm.DB }

func NewProductRepository(db *gorm.DB) repository.ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) SaveProduct(ctx context.Context, p *model.Product) error {
	row := productModel{
		ID: p.ID.String(), WorkspaceID: p.WorkspaceID.String(), Name: p.Name.String(),
		DisplayName: p.DisplayName, Description: p.Description,
		Version: p.Version, NextVersion: p.NextVersion, CreatedBy: p.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: p.CreatedAt, UpdatedAt: p.CreatedAt},
	}
	return postgres.DB(ctx, r.db).Create(&row).Error
}

func (r *ProductRepository) UpdateProduct(ctx context.Context, p *model.Product) error {
	return postgres.DB(ctx, r.db).Model(&productModel{}).Where("id = ?", p.ID.String()).Updates(map[string]any{
		"display_name": p.DisplayName, "description": p.Description,
		"version": p.Version, "next_version": p.NextVersion, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ProductRepository) FindProductByID(ctx context.Context, id kernel.ProductID) (*model.Product, error) {
	var row productModel
	err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToProduct(row)
}

func (r *ProductRepository) ListProductsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Product, error) {
	var rows []productModel
	if err := postgres.DB(ctx, r.db).Where("workspace_id = ?", wsID.String()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Product, 0, len(rows))
	for _, row := range rows {
		p, err := rowToProduct(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProductRepository) SavePlan(ctx context.Context, plan *model.ProductPlan) error {
	otherInfo, _ := json.Marshal(plan.OtherInfo)
	row := productPlanModel{
		ID: plan.ID.String(), ProductID: plan.ProductID.String(), Name: plan.Name,
		Version: plan.Version, Status: string(plan.Status), Description: plan.Description,
		Submitted: plan.Submitted, OtherInfo: string(otherInfo),
		AuditColumns: postgres.AuditColumns{CreatedAt: plan.CreatedAt, UpdatedAt: plan.CreatedAt},
	}
	return postgres.DB(ctx, r.db).Create(&row).Error
}

func (r *ProductRepository) ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductPlan, error) {
	var rows []productPlanModel
	if err := postgres.DB(ctx, r.db).Where("product_id = ?", productID.String()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.ProductPlan, 0, len(rows))
	for _, row := range rows {
		plan, err := rowToPlan(row)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func (r *ProductRepository) SaveRequirement(ctx context.Context, req *model.ProductRequirement) error {
	otherInfo, _ := json.Marshal(req.OtherInfo)
	row := productRequirementModel{
		ID: req.ID.String(), ProductID: req.ProductID.String(), ProductPlanID: req.ProductPlanID.String(),
		ProjectID: req.ProjectID.String(), ProjectName: req.ProjectName, ProjectTag: req.ProjectTag,
		BuildID: req.BuildID.String(), Description: req.Description, Status: string(req.Status),
		MulticlusterStatus: req.MulticlusterStatus, Operator: req.Operator, OtherInfo: string(otherInfo),
		AuditColumns: postgres.AuditColumns{CreatedAt: req.CreatedAt, UpdatedAt: req.CreatedAt},
	}
	return postgres.DB(ctx, r.db).Create(&row).Error
}

func (r *ProductRepository) FindRequirementByID(ctx context.Context, id kernel.ProductRequirementID) (*model.ProductRequirement, error) {
	var row productRequirementModel
	err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductRequirementNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToRequirement(row)
}

func (r *ProductRepository) FindRequirementByBuildID(ctx context.Context, buildID kernel.BuildID) (*model.ProductRequirement, error) {
	if buildID.UUID == uuid.Nil {
		return nil, domain.ErrProductRequirementNotFound
	}
	var row productRequirementModel
	err := postgres.DB(ctx, r.db).Where("build_id = ?", buildID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductRequirementNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToRequirement(row)
}

func (r *ProductRepository) UpdateRequirement(ctx context.Context, req *model.ProductRequirement) error {
	otherInfo, _ := json.Marshal(req.OtherInfo)
	return postgres.DB(ctx, r.db).Model(&productRequirementModel{}).Where("id = ?", req.ID.String()).Updates(map[string]any{
		"project_id": req.ProjectID.String(), "project_name": req.ProjectName, "project_tag": req.ProjectTag,
		"build_id": req.BuildID.String(), "description": req.Description, "status": string(req.Status),
		"multicluster_status": req.MulticlusterStatus, "operator": req.Operator,
		"other_info": string(otherInfo), "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ProductRepository) ListRequirementsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductRequirement, error) {
	var rows []productRequirementModel
	if err := postgres.DB(ctx, r.db).Where("product_id = ?", productID.String()).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.ProductRequirement, 0, len(rows))
	for _, row := range rows {
		req, err := rowToRequirement(row)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}

func rowToProduct(row productModel) (*model.Product, error) {
	name, err := model.NewProductName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseProductID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &model.Product{
		ID: id, WorkspaceID: wsID, Name: name, DisplayName: row.DisplayName,
		Description: row.Description, Version: row.Version, NextVersion: row.NextVersion,
		CreatedBy: createdBy, CreatedAt: row.CreatedAt,
	}, nil
}

func rowToPlan(row productPlanModel) (*model.ProductPlan, error) {
	id, _ := kernel.ParseProductPlanID(row.ID)
	productID, _ := kernel.ParseProductID(row.ProductID)
	otherInfo := map[string]any{}
	if row.OtherInfo != "" {
		_ = json.Unmarshal([]byte(row.OtherInfo), &otherInfo)
	}
	return &model.ProductPlan{
		ID: id, ProductID: productID, Name: row.Name, Version: row.Version,
		Status: model.ProductPlanStatus(row.Status), Description: row.Description,
		Submitted: row.Submitted, OtherInfo: otherInfo, CreatedAt: row.CreatedAt,
	}, nil
}

func rowToRequirement(row productRequirementModel) (*model.ProductRequirement, error) {
	id, _ := kernel.ParseProductRequirementID(row.ID)
	productID, _ := kernel.ParseProductID(row.ProductID)
	planID, _ := kernel.ParseProductPlanID(row.ProductPlanID)
	projectID, _ := kernel.ParseProjectID(row.ProjectID)
	buildID, _ := kernel.ParseBuildID(row.BuildID)
	otherInfo := map[string]any{}
	if row.OtherInfo != "" {
		_ = json.Unmarshal([]byte(row.OtherInfo), &otherInfo)
	}
	return &model.ProductRequirement{
		ID: id, ProductID: productID, ProductPlanID: planID, ProjectID: projectID,
		ProjectName: row.ProjectName, ProjectTag: row.ProjectTag, BuildID: buildID,
		Description: row.Description, Status: model.ProductRequirementStatus(row.Status),
		MulticlusterStatus: row.MulticlusterStatus, Operator: row.Operator,
		OtherInfo: otherInfo, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.ProductRepository = (*ProductRepository)(nil)
