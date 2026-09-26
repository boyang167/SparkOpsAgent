package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func ensureMember(ctx context.Context, checker port.WorkspaceChecker, memberID kernel.MemberID, wsID kernel.WorkspaceID) error {
	ok, err := checker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	return nil
}

type CreateProductHandler struct {
	repo      repository.ProductRepository
	wsChecker port.WorkspaceChecker
}

func NewCreateProductHandler(repo repository.ProductRepository, wsChecker port.WorkspaceChecker) *CreateProductHandler {
	return &CreateProductHandler{repo: repo, wsChecker: wsChecker}
}

func (h *CreateProductHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, rawName, displayName, description, version, nextVersion string) (*model.Product, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	name, err := model.NewProductName(rawName)
	if err != nil {
		return nil, err
	}
	p := model.NewProduct(wsID, name, displayName, description, version, nextVersion, memberID)
	if err := h.repo.SaveProduct(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

type UpdateProductHandler struct {
	repo      repository.ProductRepository
	wsChecker port.WorkspaceChecker
}

func NewUpdateProductHandler(repo repository.ProductRepository, wsChecker port.WorkspaceChecker) *UpdateProductHandler {
	return &UpdateProductHandler{repo: repo, wsChecker: wsChecker}
}

func (h *UpdateProductHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, productID kernel.ProductID, displayName, description, version, nextVersion string) (*model.Product, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	p, err := h.repo.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p.WorkspaceID != wsID {
		return nil, domain.ErrProductNotFound
	}
	p.Update(displayName, description, version, nextVersion)
	if err := h.repo.UpdateProduct(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

type CreateProductPlanHandler struct {
	repo        repository.ProductRepository
	sprints     repository.SprintRepository
	processes   appctx.SprintProcessPort
}

func NewCreateProductPlanHandler(
	repo repository.ProductRepository,
	sprints repository.SprintRepository,
	processes appctx.SprintProcessPort,
) *CreateProductPlanHandler {
	return &CreateProductPlanHandler{repo: repo, sprints: sprints, processes: processes}
}

func (h *CreateProductPlanHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, name, version, description string, otherInfo map[string]any) (*model.ProductPlan, error) {
	if _, err := h.repo.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	plan := model.NewProductPlan(productID, name, version, description, otherInfo)
	if err := h.repo.SavePlan(ctx, plan); err != nil {
		return nil, err
	}
	if h.sprints != nil && h.processes != nil {
		_, _ = service.EnsureSprintForProductPlan(ctx, plan, actor, h.sprints, h.processes)
	}
	return plan, nil
}

type CreateProductRequirementHandler struct {
	repo  repository.ProductRepository
	plans repository.ReleasePlanRepository
}

func NewCreateProductRequirementHandler(repo repository.ProductRepository, plans repository.ReleasePlanRepository) *CreateProductRequirementHandler {
	return &CreateProductRequirementHandler{repo: repo, plans: plans}
}

func (h *CreateProductRequirementHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ProductPlanID, projectID kernel.ProjectID, projectName, projectTag, description, operator string, otherInfo map[string]any) (*model.ProductRequirement, error) {
	if _, err := h.repo.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	req := model.NewProductRequirement(productID, planID, projectID, projectName, projectTag, description, operator, otherInfo)
	if err := h.repo.SaveRequirement(ctx, req); err != nil {
		return nil, err
	}
	if h.plans != nil {
		now := time.Now().UTC()
		name := strings.TrimSpace(projectName)
		if name == "" {
			name = "Requirement " + req.ID.String()[:8]
		}
		actor := kernel.NewMemberID()
		releasePlan := model.NewReleasePlan(productID, name, description, actor, actor, now, now.Add(7*24*time.Hour), model.ApprovalConfig{Enabled: false})
		releasePlan.ProductPlanID = &planID
		if err := h.plans.SavePlan(ctx, releasePlan); err == nil {
			text := description
			if projectTag != "" {
				text = "Branch/tag: " + projectTag + "\n" + text
			}
			job := &model.ReleasePlanJob{
				ID: kernel.NewReleasePlanJobID(), ReleasePlanID: releasePlan.ID, SortOrder: 0,
				Name: "Requirement checklist", Type: model.ReleasePlanJobTypeText, OwnerID: actor,
				Status: model.ReleasePlanJobStatusTodo, TextContent: text, CreatedAt: now, UpdatedAt: now,
			}
			_ = h.plans.SaveJob(ctx, job)
			req.OtherInfo["release_plan_id"] = releasePlan.ID.String()
			_ = h.repo.UpdateRequirement(ctx, req)
		}
	}
	return req, nil
}
