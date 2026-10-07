package service

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
)

func BackfillReleasePlanVersions(ctx context.Context, plans repository.ReleasePlanRepository, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 200
	}
	planIDs, err := plans.ListPlanIDsWithoutVersions(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	backfilled := 0
	for _, planID := range planIDs {
		plan, err := plans.FindPlanByID(ctx, planID)
		if err != nil {
			return backfilled, err
		}
		snapshot := model.VersionSnapshotFromPlan(plan)
		version := model.NewReleasePlanVersion(
			plan.ID, 1, 0, plan.CreatedBy, "backfill", "plan", "历史回填", snapshot,
		)
		version.CreatedAt = plan.CreatedAt
		if err := plans.SaveVersion(ctx, version); err != nil {
			return backfilled, err
		}
		if plan.Version <= 0 {
			if err := plans.UpdatePlanVersion(ctx, plan.ID, 1); err != nil {
				return backfilled, err
			}
		}
		backfilled++
	}
	return backfilled, nil
}
