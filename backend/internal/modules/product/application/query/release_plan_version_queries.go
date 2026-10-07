package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListReleasePlanVersionsHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewListReleasePlanVersionsHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ListReleasePlanVersionsHandler {
	return &ListReleasePlanVersionsHandler{products: products, plans: plans}
}

func (h *ListReleasePlanVersionsHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) ([]map[string]any, error) {
	if err := h.ensurePlan(ctx, productID, planID); err != nil {
		return nil, err
	}
	versions, err := h.plans.ListVersionsByPlan(ctx, planID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(versions))
	for _, item := range versions {
		out = append(out, releasePlanVersionSummary(item))
	}
	return out, nil
}

type GetReleasePlanVersionDiffHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewGetReleasePlanVersionDiffHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *GetReleasePlanVersionDiffHandler {
	return &GetReleasePlanVersionDiffHandler{products: products, plans: plans}
}

func (h *GetReleasePlanVersionDiffHandler) Handle(
	ctx context.Context,
	productID kernel.ProductID,
	planID kernel.ReleasePlanID,
	version int,
	compareVersion *int,
) (model.ReleasePlanVersionDiffResponse, error) {
	if err := h.ensurePlan(ctx, productID, planID); err != nil {
		return model.ReleasePlanVersionDiffResponse{}, err
	}
	current, err := h.plans.GetVersion(ctx, planID, version)
	if err != nil {
		return model.ReleasePlanVersionDiffResponse{}, err
	}
	baseVersion := current.PreviousVersion
	if compareVersion != nil {
		baseVersion = *compareVersion
	}
	var before model.ReleasePlanVersionSnapshot
	if baseVersion > 0 {
		previous, err := h.plans.GetVersion(ctx, planID, baseVersion)
		if err != nil {
			return model.ReleasePlanVersionDiffResponse{}, err
		}
		before = previous.Snapshot
	}
	return model.DiffReleasePlanVersionSnapshots(planID.String(), current.Version, baseVersion, before, current.Snapshot), nil
}

func (h *ListReleasePlanVersionsHandler) ensurePlan(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) error {
	_, err := h.ensurePlanEntity(ctx, productID, planID)
	return err
}

func (h *GetReleasePlanVersionDiffHandler) ensurePlan(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) error {
	_, err := h.ensurePlanEntity(ctx, productID, planID)
	return err
}

func ensurePlanEntity(ctx context.Context, products repository.ProductRepository, plans repository.ReleasePlanRepository, productID kernel.ProductID, planID kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	if _, err := products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	plan, err := plans.FindPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan.ProductID != productID {
		return nil, domain.ErrReleasePlanNotFound
	}
	return plan, nil
}

func (h *ListReleasePlanVersionsHandler) ensurePlanEntity(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	return ensurePlanEntity(ctx, h.products, h.plans, productID, planID)
}

func (h *GetReleasePlanVersionDiffHandler) ensurePlanEntity(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	return ensurePlanEntity(ctx, h.products, h.plans, productID, planID)
}

func releasePlanVersionSummary(item model.ReleasePlanVersion) map[string]any {
	return map[string]any{
		"version":          item.Version,
		"previous_version": item.PreviousVersion,
		"actor_id":         item.ActorID.String(),
		"verb":             item.Verb,
		"section_key":      item.SectionKey,
		"section_name":     item.SectionName,
		"created_at":       item.CreatedAt.Format(time.RFC3339),
	}
}
