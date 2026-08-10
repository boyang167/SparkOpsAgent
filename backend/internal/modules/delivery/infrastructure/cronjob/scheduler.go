package cronjob

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
)

type Scheduler struct {
	repo     *persistence.CronJobRepository
	trigger  *command.TriggerBuildHandler
	logger   *zap.Logger
	interval time.Duration
	mu       sync.Mutex
	lastRun  map[string]time.Time
}

func NewScheduler(
	repo *persistence.CronJobRepository,
	trigger *command.TriggerBuildHandler,
	logger *zap.Logger,
) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scheduler{
		repo: repo, trigger: trigger, logger: logger,
		interval: time.Minute, lastRun: make(map[string]time.Time),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s.repo == nil || s.trigger == nil {
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
	jobs, err := s.repo.ListEnabled(ctx)
	if err != nil {
		s.logger.Warn("cronjob list failed", zap.Error(err))
		return
	}
	now := time.Now()
	for _, job := range jobs {
		schedule, err := cron.ParseStandard(job.Schedule)
		if err != nil {
			s.logger.Warn("invalid cron schedule", zap.String("id", job.ID.String()), zap.Error(err))
			continue
		}
		s.mu.Lock()
		last := s.lastRun[job.ID.String()]
		if last.IsZero() {
			last = now.Add(-time.Minute)
		}
		next := schedule.Next(last)
		shouldRun := !next.After(now)
		if shouldRun {
			s.lastRun[job.ID.String()] = now
		}
		s.mu.Unlock()
		if !shouldRun {
			continue
		}
		if _, err := s.trigger.TriggerAutomated(ctx, job.ProjectID, job.Branch); err != nil {
			s.logger.Warn("cronjob trigger failed",
				zap.String("cronjob_id", job.ID.String()),
				zap.String("project_id", job.ProjectID.String()),
				zap.Error(err),
			)
			continue
		}
		s.logger.Info("cronjob triggered build",
			zap.String("cronjob_id", job.ID.String()),
			zap.String("project_id", job.ProjectID.String()),
			zap.String("branch", job.Branch),
		)
	}
}
