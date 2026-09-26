package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanScheduler struct {
	plans    repository.ReleasePlanRepository
	executor *ReleasePlanExecutor
	logger   *zap.Logger
	interval time.Duration
}

func NewReleasePlanScheduler(
	plans repository.ReleasePlanRepository,
	executor *ReleasePlanExecutor,
	logger *zap.Logger,
) *ReleasePlanScheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ReleasePlanScheduler{
		plans: plans, executor: executor, logger: logger, interval: time.Minute,
	}
}

func (s *ReleasePlanScheduler) Start(ctx context.Context) {
	if s == nil || s.plans == nil || s.executor == nil {
		return
	}
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		s.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *ReleasePlanScheduler) tick(ctx context.Context) {
	now := time.Now().UTC()
	plans, err := s.plans.ListDueScheduledPlans(ctx, now)
	if err != nil {
		s.logger.Warn("release plan scheduler list failed", zap.Error(err))
		return
	}
	for _, plan := range plans {
		if err := s.executor.Advance(ctx, plan, kernel.MemberID{}); err != nil {
			s.logger.Warn("release plan scheduled advance failed",
				zap.String("plan_id", plan.ID.String()), zap.Error(err))
		}
	}
}

func ShouldAdvancePlan(plan *model.ReleasePlan, now time.Time) bool {
	if plan == nil || plan.Status != model.ReleasePlanStatusExecuting {
		return false
	}
	return !plan.ScheduledExecutionPending(now)
}
