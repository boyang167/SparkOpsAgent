package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func MapProductPlanStatusToSprint(status model.ProductPlanStatus) model.SprintStatus {
	switch status {
	case model.ProductPlanStatusActive:
		return model.SprintStatusActive
	case model.ProductPlanStatusCompleted:
		return model.SprintStatusCompleted
	default:
		return model.SprintStatusPlanning
	}
}

func EnsureSprintForProductPlan(
	ctx context.Context,
	plan *model.ProductPlan,
	actor kernel.MemberID,
	sprints repository.SprintRepository,
	processes appctx.SprintProcessPort,
) (*model.Sprint, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan required")
	}
	existing, err := sprints.FindSprintByLegacyPlanID(ctx, plan.ID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrSprintNotFound) {
		return nil, err
	}
	process, err := processes.GetOrSeedDefault(ctx, plan.ProductID, actor)
	if err != nil {
		return nil, err
	}
	name := plan.Name
	if name == "" {
		name = plan.Version
	}
	sprint := model.NewSprint(plan.ProductID, process, name, plan.Description, nil, actor)
	sprint.Status = MapProductPlanStatusToSprint(plan.Status)
	sprint.LegacyPlanID = &plan.ID
	if err := sprints.SaveSprint(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}
