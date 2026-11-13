package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const ReconcilerLeaseName = "filesystem-reconciler"

var ErrLeaseNotOwned = errors.New("filesystem reconciler lease not owned")

type ReconcilerConfig struct {
	Interval      time.Duration
	LeaseTTL      time.Duration
	BatchSize     int
	GCGracePeriod time.Duration
}

func (c ReconcilerConfig) withDefaults() ReconcilerConfig {
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.LeaseTTL <= 0 {
		c.LeaseTTL = 90 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.GCGracePeriod <= 0 {
		c.GCGracePeriod = 168 * time.Hour
	}
	return c
}

type ReconcileSummary struct {
	ExpiredReservationsReleased int `json:"expired_reservations_released"`
	StaleTransfersExpired       int `json:"stale_transfers_expired"`
	BlobReplicaIssues           int `json:"blob_replica_issues"`
	BacklogJobsClaimed          int `json:"backlog_jobs_claimed"`
}

type Lease interface {
	TryAcquireOrRenew(ctx context.Context, name, owner string, ttl time.Duration, now time.Time) (bool, error)
	RecordPass(ctx context.Context, name, owner string, summary ReconcileSummary, now time.Time) error
}

type WorkRunner interface {
	RunPass(ctx context.Context, cfg ReconcilerConfig, now time.Time) (ReconcileSummary, error)
}

type Reconciler struct {
	owner  string
	lease  Lease
	work   WorkRunner
	config ReconcilerConfig
	clock  func() time.Time
	report func(error)
}

func NewReconciler(owner string, lease Lease, work WorkRunner, config ReconcilerConfig) *Reconciler {
	if lease == nil {
		panic("filesystem reconciler lease is required")
	}
	if work == nil {
		panic("filesystem reconciler work runner is required")
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		panic("filesystem reconciler owner is required")
	}
	return &Reconciler{
		owner: owner, lease: lease, work: work,
		config: config.withDefaults(),
		clock:  func() time.Time { return time.Now().UTC() },
	}
}

func (r *Reconciler) RunOnce(ctx context.Context) error {
	now := r.clock()
	acquired, err := r.lease.TryAcquireOrRenew(ctx, ReconcilerLeaseName, r.owner, r.config.LeaseTTL, now)
	if err != nil {
		r.reportError(err)
		return err
	}
	if !acquired {
		return ErrLeaseNotOwned
	}

	summary, err := r.work.RunPass(ctx, r.config, now)
	if err != nil {
		r.reportError(err)
		return err
	}
	if err := r.lease.RecordPass(ctx, ReconcilerLeaseName, r.owner, summary, now); err != nil {
		r.reportError(err)
		return err
	}
	return nil
}

func (r *Reconciler) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.RunOnce(ctx)
		interval := r.config.Interval
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

func (r *Reconciler) SetClock(clock func() time.Time) {
	if clock != nil {
		r.clock = clock
	}
}

func (r *Reconciler) SetErrorReporter(report func(error)) {
	r.report = report
}

func (r *Reconciler) reportError(err error) {
	if r.report != nil && err != nil {
		r.report(err)
	}
}

func MarshalSummary(summary ReconcileSummary) (string, error) {
	body, err := json.Marshal(summary)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
