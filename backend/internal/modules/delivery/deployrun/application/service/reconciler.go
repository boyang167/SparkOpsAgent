package service

import (
	"context"
	"errors"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
)

type Reconciler struct {
	repo                 repository.DeploymentRunRepository
	publisher            RunPublisher
	report               func(error)
	consumeFailureBudget bool
}

func NewReconciler(repo repository.DeploymentRunRepository, publisher RunPublisher, reporters ...func(error)) *Reconciler {
	return newReconciler(repo, publisher, true, reporters...)
}

func NewReliableReconciler(repo repository.DeploymentRunRepository, publisher RunPublisher, reporters ...func(error)) *Reconciler {
	return newReconciler(repo, publisher, false, reporters...)
}

func newReconciler(
	repo repository.DeploymentRunRepository,
	publisher RunPublisher,
	consumeFailureBudget bool,
	reporters ...func(error),
) *Reconciler {
	reconciler := &Reconciler{
		repo: repo, publisher: publisher, consumeFailureBudget: consumeFailureBudget,
	}
	if len(reporters) > 0 {
		reconciler.report = reporters[0]
	}
	return reconciler
}

func (r *Reconciler) RunOnce(ctx context.Context) error {
	runs, err := r.repo.ListActive(ctx)
	if err != nil {
		r.reportError(err)
		return err
	}
	var result error
	for _, run := range runs {
		if err := r.publisher.PublishRun(ctx, run); err != nil {
			r.reportError(err)
			result = errors.Join(result, err)
			if !r.consumeFailureBudget {
				continue
			}
			if _, updateErr := r.repo.RecordInfrastructureFailure(
				ctx, run.ID, run.Version, "deployment message retry budget exhausted",
			); updateErr != nil {
				r.reportError(updateErr)
				result = errors.Join(result, updateErr)
			}
		}
	}
	return result
}

func (r *Reconciler) Start(ctx context.Context, interval time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.RunOnce(ctx)
		if interval <= 0 {
			<-ctx.Done()
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.RunOnce(ctx)
			}
		}
	}()
	return done
}

func (r *Reconciler) reportError(err error) {
	if r.report != nil {
		r.report(err)
	}
}
