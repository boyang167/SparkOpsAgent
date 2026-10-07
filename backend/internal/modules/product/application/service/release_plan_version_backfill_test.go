package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type releasePlanVersionBackfillRepo struct {
	plans map[kernel.ReleasePlanID]*model.ReleasePlan
	versions []model.ReleasePlanVersion
}

func (r *releasePlanVersionBackfillRepo) SavePlan(context.Context, *model.ReleasePlan) error { return nil }
func (r *releasePlanVersionBackfillRepo) UpdatePlan(context.Context, *model.ReleasePlan) error { return nil }
func (r *releasePlanVersionBackfillRepo) DeletePlan(context.Context, kernel.ReleasePlanID) error { return nil }
func (r *releasePlanVersionBackfillRepo) FindPlanByID(_ context.Context, id kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	plan, ok := r.plans[id]
	if !ok {
		return nil, nil
	}
	return plan, nil
}
func (r *releasePlanVersionBackfillRepo) ListPlansByProduct(context.Context, kernel.ProductID) ([]*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) ListPlansByProductFiltered(context.Context, kernel.ProductID, *kernel.SprintID, repository.ReleasePlanListFilter) ([]*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) ListPlansByProductAndSprint(context.Context, kernel.ProductID, kernel.SprintID) ([]*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) ListPlansByWorkspace(context.Context, kernel.WorkspaceID, repository.ReleasePlanListFilter) ([]*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) HasBlockingPlansBySprint(context.Context, kernel.SprintID) (bool, error) {
	return false, nil
}
func (r *releasePlanVersionBackfillRepo) ListBlockingPlansBySprint(context.Context, kernel.SprintID) ([]*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) SaveJob(context.Context, *model.ReleasePlanJob) error { return nil }
func (r *releasePlanVersionBackfillRepo) UpdateJob(context.Context, *model.ReleasePlanJob) error { return nil }
func (r *releasePlanVersionBackfillRepo) DeleteJob(context.Context, kernel.ReleasePlanJobID) error { return nil }
func (r *releasePlanVersionBackfillRepo) ListJobsByPlan(context.Context, kernel.ReleasePlanID) ([]model.ReleasePlanJob, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) SaveAuditLog(context.Context, *model.ReleasePlanAuditLog) error { return nil }
func (r *releasePlanVersionBackfillRepo) ListAuditLogs(context.Context, kernel.ReleasePlanID, int) ([]model.ReleasePlanAuditLog, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) SaveVersion(_ context.Context, version *model.ReleasePlanVersion) error {
	r.versions = append(r.versions, *version)
	return nil
}
func (r *releasePlanVersionBackfillRepo) ListVersionsByPlan(context.Context, kernel.ReleasePlanID, int) ([]model.ReleasePlanVersion, error) {
	return r.versions, nil
}
func (r *releasePlanVersionBackfillRepo) GetVersion(context.Context, kernel.ReleasePlanID, int) (*model.ReleasePlanVersion, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) UpdatePlanVersion(_ context.Context, planID kernel.ReleasePlanID, version int) error {
	if plan, ok := r.plans[planID]; ok {
		plan.Version = version
	}
	return nil
}
func (r *releasePlanVersionBackfillRepo) ListPlanIDsWithoutVersions(_ context.Context, limit int) ([]kernel.ReleasePlanID, error) {
	if len(r.versions) > 0 {
		return nil, nil
	}
	ids := make([]kernel.ReleasePlanID, 0, len(r.plans))
	for id := range r.plans {
		ids = append(ids, id)
		if limit > 0 && len(ids) >= limit {
			break
		}
	}
	return ids, nil
}
func (r *releasePlanVersionBackfillRepo) FindJobByWorkflowRunID(context.Context, kernel.ProductWorkflowRunID) (*model.ReleasePlanJob, *model.ReleasePlan, error) {
	return nil, nil, nil
}
func (r *releasePlanVersionBackfillRepo) FindPlanByApprovalToken(context.Context, string) (*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) FindPlanByHookToken(context.Context, string) (*model.ReleasePlan, error) {
	return nil, nil
}
func (r *releasePlanVersionBackfillRepo) ListDueScheduledPlans(context.Context, time.Time) ([]*model.ReleasePlan, error) {
	return nil, nil
}

func TestBackfillReleasePlanVersions(t *testing.T) {
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: false},
	)
	repo := &releasePlanVersionBackfillRepo{plans: map[kernel.ReleasePlanID]*model.ReleasePlan{plan.ID: plan}}

	count, err := service.BackfillReleasePlanVersions(context.Background(), repo, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Len(t, repo.versions, 1)
	require.Equal(t, 1, repo.versions[0].Version)
	require.Equal(t, "backfill", repo.versions[0].Verb)
	require.Equal(t, 1, plan.Version)
}
