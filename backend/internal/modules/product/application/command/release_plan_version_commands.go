package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func releasePlanVersionSectionName(verb string) string {
	switch verb {
	case "create", "create_from_template", "copy":
		return "发布计划"
	case "update":
		return "基础信息"
	case "add_job", "add_jobs_from_work_items":
		return "发布项"
	case "update_job":
		return "发布项"
	case "delete_job":
		return "发布项"
	case "reorder_jobs":
		return "发布项顺序"
	case "finish_planning":
		return "完成规划"
	case "reopen_planning":
		return "重新编辑"
	default:
		return "发布计划"
	}
}

func persistReleasePlanVersion(
	ctx context.Context,
	plans repository.ReleasePlanRepository,
	plan *model.ReleasePlan,
	actor kernel.MemberID,
	verb string,
) error {
	snapshot := model.VersionSnapshotFromPlan(plan)
	previousVersion := plan.Version
	nextVersion := previousVersion
	if previousVersion <= 0 {
		nextVersion = 1
	} else {
		nextVersion = previousVersion + 1
	}
	version := model.NewReleasePlanVersion(
		plan.ID, nextVersion, previousVersion, actor, verb, "plan",
		releasePlanVersionSectionName(verb), snapshot,
	)
	if err := plans.SaveVersion(ctx, version); err != nil {
		return err
	}
	plan.Version = nextVersion
	return plans.UpdatePlanVersion(ctx, plan.ID, nextVersion)
}

func recordPlanVersionAfterReload(
	ctx context.Context,
	plans repository.ReleasePlanRepository,
	planID kernel.ReleasePlanID,
	actor kernel.MemberID,
	verb string,
) error {
	plan, err := plans.FindPlanByID(ctx, planID)
	if err != nil {
		return err
	}
	return persistReleasePlanVersion(ctx, plans, plan, actor, verb)
}
