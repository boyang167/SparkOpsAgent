package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type CreateEnvironmentInput struct {
	Name       string
	EnvType    model.EnvironmentType
	ClusterID  kernel.ClusterInfoID
	Namespace  string
	Variables  map[string]string
	ProjectIDs []kernel.ProjectID // 为空则纳入产品下全部项目
}

type CreateEnvironmentHandler struct {
	products     repository.ProductRepository
	environments repository.EnvironmentRepository
	catalog      port.ProjectCatalogPort
	db           *gorm.DB
}

func NewCreateEnvironmentHandler(
	products repository.ProductRepository,
	environments repository.EnvironmentRepository,
	catalog port.ProjectCatalogPort,
	db *gorm.DB,
) *CreateEnvironmentHandler {
	return &CreateEnvironmentHandler{products: products, environments: environments, catalog: catalog, db: db}
}

func (h *CreateEnvironmentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	actor kernel.MemberID,
	input CreateEnvironmentInput,
) (*model.Environment, error) {
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.WorkspaceID != wsID {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	name := strings.TrimSpace(input.Name)
	namespace := strings.TrimSpace(input.Namespace)
	if name == "" || namespace == "" || input.ClusterID.UUID == uuid.Nil {
		return nil, domain.ErrInvalidEnvironmentInput
	}
	ok, err := persistence.ClusterAuthorized(ctx, h.db, wsID, input.ClusterID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrEnvironmentClusterNotAuthorized
	}
	env := model.NewEnvironment(productID, wsID, name, namespace, input.EnvType, input.ClusterID, input.Variables, actor)
	if err := h.environments.SaveEnvironment(ctx, env); err != nil {
		return nil, err
	}
	projectIDs := input.ProjectIDs
	if len(projectIDs) == 0 {
		projects, listErr := h.catalog.ListProjectsByProduct(ctx, wsID, productID)
		if listErr != nil {
			return nil, listErr
		}
		projectIDs = make([]kernel.ProjectID, 0, len(projects))
		for _, p := range projects {
			pid, parseErr := kernel.ParseProjectID(p.ID)
			if parseErr != nil {
				continue
			}
			projectIDs = append(projectIDs, pid)
		}
	} else {
		// 仅允许纳入本产品下的项目
		projects, listErr := h.catalog.ListProjectsByProduct(ctx, wsID, productID)
		if listErr != nil {
			return nil, listErr
		}
		allowed := make(map[string]struct{}, len(projects))
		for _, p := range projects {
			allowed[p.ID] = struct{}{}
		}
		filtered := make([]kernel.ProjectID, 0, len(projectIDs))
		for _, pid := range projectIDs {
			if _, ok := allowed[pid.String()]; ok {
				filtered = append(filtered, pid)
			}
		}
		projectIDs = filtered
	}
	if err := h.environments.EnsureEnvironmentProjects(ctx, env.ID, projectIDs); err != nil {
		return nil, err
	}
	return env, nil
}

type UpdateEnvironmentInput struct {
	Name      *string
	EnvType   *model.EnvironmentType
	ClusterID *kernel.ClusterInfoID
	Namespace *string
	Variables map[string]string
	Status    *model.EnvironmentStatus
}

type UpdateEnvironmentHandler struct {
	products     repository.ProductRepository
	environments repository.EnvironmentRepository
	db           *gorm.DB
}

func NewUpdateEnvironmentHandler(
	products repository.ProductRepository,
	environments repository.EnvironmentRepository,
	db *gorm.DB,
) *UpdateEnvironmentHandler {
	return &UpdateEnvironmentHandler{products: products, environments: environments, db: db}
}

func (h *UpdateEnvironmentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	envID kernel.EnvironmentID,
	input UpdateEnvironmentInput,
) (*model.Environment, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	env, err := h.environments.FindEnvironmentByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if env.ProductID != productID || env.WorkspaceID != wsID {
		return nil, domain.ErrEnvironmentNotFound
	}
	if input.Name != nil {
		env.Name = strings.TrimSpace(*input.Name)
	}
	if input.EnvType != nil {
		env.EnvType = *input.EnvType
	}
	if input.ClusterID != nil {
		ok, checkErr := persistence.ClusterAuthorized(ctx, h.db, wsID, *input.ClusterID)
		if checkErr != nil {
			return nil, checkErr
		}
		if !ok {
			return nil, domain.ErrEnvironmentClusterNotAuthorized
		}
		env.ClusterID = *input.ClusterID
	}
	if input.Namespace != nil {
		env.Namespace = strings.TrimSpace(*input.Namespace)
	}
	if input.Variables != nil {
		env.Variables = input.Variables
	}
	if input.Status != nil {
		env.Status = *input.Status
	}
	env.UpdatedAt = time.Now().UTC()
	if err := h.environments.SaveEnvironment(ctx, env); err != nil {
		return nil, err
	}
	return env, nil
}

type DeleteEnvironmentHandler struct {
	products     repository.ProductRepository
	environments repository.EnvironmentRepository
}

func NewDeleteEnvironmentHandler(products repository.ProductRepository, environments repository.EnvironmentRepository) *DeleteEnvironmentHandler {
	return &DeleteEnvironmentHandler{products: products, environments: environments}
}

func (h *DeleteEnvironmentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID, envID kernel.EnvironmentID) error {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return err
	}
	env, err := h.environments.FindEnvironmentByID(ctx, envID)
	if err != nil {
		return err
	}
	if env.ProductID != productID || env.WorkspaceID != wsID {
		return domain.ErrEnvironmentNotFound
	}
	return h.environments.DeleteEnvironment(ctx, envID)
}
