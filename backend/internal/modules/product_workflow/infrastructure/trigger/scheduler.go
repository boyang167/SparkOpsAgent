package trigger

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
)

type Scheduler struct {
	repo     repository.ProductWorkflowRepository
	start    *command.StartWorkflowRunHandler
	logger   *zap.Logger
	interval time.Duration
	mu       sync.Mutex
	lastRun  map[string]time.Time
}

func NewScheduler(repo repository.ProductWorkflowRepository, start *command.StartWorkflowRunHandler, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scheduler{
		repo: repo, start: start, logger: logger,
		interval: time.Minute, lastRun: map[string]time.Time{},
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.repo == nil || s.start == nil {
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

func (s *Scheduler) tick(ctx context.Context) {
	items, err := s.repo.ListAllWorkflows(ctx)
	if err != nil {
		s.logger.Warn("workflow cron list failed", zap.Error(err))
		return
	}
	now := time.Now()
	for _, wf := range items {
		if wf == nil || wf.Status != model.WorkflowStatusActive {
			continue
		}
		for _, item := range wf.Triggers {
			if item.Type != model.TriggerCron || !item.Active() {
				continue
			}
			schedule, err := cron.ParseStandard(item.Schedule)
			if err != nil {
				continue
			}
			key := wf.ID.String() + ":" + item.ID
			s.mu.Lock()
			last := s.lastRun[key]
			if last.IsZero() {
				last = now.Add(-time.Minute)
			}
			shouldRun := !schedule.Next(last).After(now)
			if shouldRun {
				s.lastRun[key] = now
			}
			s.mu.Unlock()
			if !shouldRun {
				continue
			}
			if _, err := s.start.StartFromCron(ctx, wf); err != nil {
				s.logger.Warn("workflow cron trigger failed", zap.String("workflow_id", wf.ID.String()), zap.Error(err))
				continue
			}
			s.logger.Info("workflow cron triggered", zap.String("workflow_id", wf.ID.String()), zap.String("schedule", item.Schedule))
		}
	}
	s.start.ExpireRuns(ctx)
}
