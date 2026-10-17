package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

type dispatcherStore struct {
	mu          sync.Mutex
	records     []persistence.OutboxRecord
	claimed     map[string]string
	published   []string
	rescheduled []string
	markErr     error
	claimErr    error
	stats       persistence.OutboxStats
	statsErr    error
}

func (s *dispatcherStore) Insert(context.Context, events.Envelope) error { return nil }

func (s *dispatcherStore) Claim(_ context.Context, owner string, limit int, _ time.Time, _ time.Duration) ([]persistence.OutboxRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	if s.claimed == nil {
		s.claimed = make(map[string]string)
	}
	var result []persistence.OutboxRecord
	for _, record := range s.records {
		if len(result) == limit {
			break
		}
		if _, exists := s.claimed[record.ID]; exists {
			continue
		}
		s.claimed[record.ID] = owner
		record.LockedBy = owner
		result = append(result, record)
	}
	return result, nil
}

func (s *dispatcherStore) MarkPublished(_ context.Context, id, owner string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	if s.markErr != nil {
		return s.markErr
	}
	s.published = append(s.published, id)
	return nil
}

func (s *dispatcherStore) Stats(context.Context, time.Time) (persistence.OutboxStats, error) {
	return s.stats, s.statsErr
}

func (s *dispatcherStore) Reschedule(_ context.Context, id, owner, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	s.rescheduled = append(s.rescheduled, id)
	delete(s.claimed, id)
	return nil
}

type fakeEventTransport struct {
	mu          sync.Mutex
	publishErr  error
	published   []events.Envelope
	retries     []RetryDelay
	deadLetters []string
	declared    []Subscription
	consumed    []Subscription
	closed      int
}

func (t *fakeEventTransport) Declare(_ context.Context, subscriptions []Subscription) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.declared = append([]Subscription(nil), subscriptions...)
	return nil
}

func (t *fakeEventTransport) Publish(_ context.Context, envelope events.Envelope) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.published = append(t.published, envelope)
	return t.publishErr
}

func (t *fakeEventTransport) Consume(_ context.Context, subscription Subscription, _ DeliveryHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.consumed = append(t.consumed, subscription)
	return nil
}

func (t *fakeEventTransport) PublishRetry(_ context.Context, _ Subscription, _ events.Envelope, delay RetryDelay, _ int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.retries = append(t.retries, delay)
	return t.publishErr
}

func (t *fakeEventTransport) PublishDeadLetter(_ context.Context, _ Subscription, _ events.Envelope, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.deadLetters = append(t.deadLetters, reason)
	return t.publishErr
}

func (t *fakeEventTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed++
	return nil
}

func TestDispatcherLeavesRecordRetryableWhenPublishConfirmFails(t *testing.T) {
	record := dispatcherRecord("event-1")
	store := &dispatcherStore{records: []persistence.OutboxRecord{record}}
	transport := &fakeEventTransport{publishErr: errors.New("confirm failed")}
	dispatcher := NewOutboxDispatcher(store, transport, OutboxDispatcherConfig{
		Owner: "worker-a", BatchSize: 10, Lease: time.Minute,
	}, fixedClock{now: record.OccurredAt}, nil)

	err := dispatcher.DispatchOnce(context.Background())

	require.ErrorContains(t, err, "confirm failed")
	require.Equal(t, []string{record.ID}, store.rescheduled)
	require.Empty(t, store.published)
}

func TestDispatcherMayRepublishAfterConfirmBeforeMarkPublished(t *testing.T) {
	record := dispatcherRecord("stable-event-id")
	store := &dispatcherStore{
		records: []persistence.OutboxRecord{record},
		markErr: errors.New("database unavailable after confirm"),
	}
	transport := &fakeEventTransport{}
	dispatcher := NewOutboxDispatcher(store, transport, OutboxDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute,
	}, fixedClock{now: record.OccurredAt}, nil)

	require.ErrorContains(t, dispatcher.DispatchOnce(context.Background()), "database unavailable")
	store.mu.Lock()
	delete(store.claimed, record.ID) // simulate lease recovery after confirm-before-mark crash
	store.markErr = nil
	store.mu.Unlock()
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))

	require.Len(t, transport.published, 2)
	require.Equal(t, record.ID, transport.published[0].ID)
	require.Equal(t, record.ID, transport.published[1].ID)
}

func TestDispatcherClaimsDisjointBatchesAcrossInstances(t *testing.T) {
	store := &dispatcherStore{records: []persistence.OutboxRecord{
		dispatcherRecord("event-1"),
		dispatcherRecord("event-2"),
	}}
	firstTransport := &fakeEventTransport{}
	secondTransport := &fakeEventTransport{}
	now := fixedClock{now: time.Now().UTC()}
	first := NewOutboxDispatcher(store, firstTransport, OutboxDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute,
	}, now, nil)
	second := NewOutboxDispatcher(store, secondTransport, OutboxDispatcherConfig{
		Owner: "worker-b", BatchSize: 1, Lease: time.Minute,
	}, now, nil)

	require.NoError(t, first.DispatchOnce(context.Background()))
	require.NoError(t, second.DispatchOnce(context.Background()))

	require.Len(t, firstTransport.published, 1)
	require.Len(t, secondTransport.published, 1)
	require.NotEqual(t, firstTransport.published[0].ID, secondTransport.published[0].ID)
}

func TestDispatcherCloseStopsClaimsAndWaitsForInflightConfirm(t *testing.T) {
	record := dispatcherRecord("event-1")
	store := &dispatcherStore{records: []persistence.OutboxRecord{record}}
	transport := &blockingPublishTransport{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	dispatcher := NewOutboxDispatcher(store, transport, OutboxDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute, PollInterval: time.Hour,
	}, fixedClock{now: record.OccurredAt}, newDispatcherTestMetrics(t))
	require.NoError(t, dispatcher.Start(context.Background()))
	<-transport.started

	closed := make(chan struct{})
	go func() {
		_ = dispatcher.Close()
		close(closed)
	}()
	select {
	case <-transport.cancelled:
		t.Fatal("Close cancelled an in-flight publisher confirm")
	case <-closed:
		t.Fatal("Close returned before the in-flight publisher confirm")
	case <-time.After(20 * time.Millisecond):
	}
	close(transport.release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after the publisher confirm completed")
	}
}

func TestDispatcherStartValidatesRequiredDependenciesBeforeLaunching(t *testing.T) {
	config := OutboxDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute, PollInterval: time.Millisecond,
	}
	validStore := &dispatcherStore{}
	validTransport := &fakeEventTransport{}
	validMetrics := newDispatcherTestMetrics(t)
	tests := []struct {
		name      string
		store     persistence.OutboxStore
		transport EventTransport
		config    OutboxDispatcherConfig
		metrics   *Metrics
		wantErr   string
	}{
		{
			name: "store", transport: validTransport, config: config,
			metrics: validMetrics, wantErr: "store",
		},
		{
			name: "transport", store: validStore, config: config,
			metrics: validMetrics, wantErr: "transport",
		},
		{
			name: "owner", store: validStore, transport: validTransport,
			config:  OutboxDispatcherConfig{BatchSize: 1, Lease: time.Minute},
			metrics: validMetrics,
			wantErr: "owner",
		},
		{
			name: "metrics", store: validStore, transport: validTransport,
			config: config, wantErr: "metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatcher := NewOutboxDispatcher(
				tt.store, tt.transport, tt.config, fixedClock{}, tt.metrics,
			)
			require.ErrorContains(t, dispatcher.Start(context.Background()), tt.wantErr)
			require.NoError(t, dispatcher.Close())
		})
	}
}

func newDispatcherTestMetrics(t *testing.T) *Metrics {
	t.Helper()
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	return metrics
}

func TestDispatcherUpdatesPublishAndBacklogMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	require.NoError(t, err)
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	store := &dispatcherStore{
		records: []persistence.OutboxRecord{dispatcherRecord("event-1")},
		stats: persistence.OutboxStats{
			Pending: 3, OldestAge: 2 * time.Minute,
		},
	}
	dispatcher := NewOutboxDispatcher(
		store,
		&fakeEventTransport{},
		OutboxDispatcherConfig{Owner: "worker-a", BatchSize: 1, Lease: time.Minute},
		fixedClock{now: now},
		metrics,
	)

	require.NoError(t, dispatcher.DispatchOnce(context.Background()))

	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventPublish.WithLabelValues("success")))
	require.Equal(t, float64(3), prometheusMetricValue(t, metrics.OutboxPending))
	require.Equal(t, float64(120), prometheusMetricValue(t, metrics.OutboxOldestSeconds))
}

func TestDispatcherStatsFailureDoesNotBlockReliablePublish(t *testing.T) {
	metrics := newDispatcherTestMetrics(t)
	record := dispatcherRecord("event-1")
	store := &dispatcherStore{
		records:  []persistence.OutboxRecord{record},
		statsErr: errors.New("stats query failed"),
	}
	transport := &fakeEventTransport{}
	dispatcher := NewOutboxDispatcher(
		store,
		transport,
		OutboxDispatcherConfig{Owner: "worker-a", BatchSize: 1, Lease: time.Minute},
		fixedClock{now: record.OccurredAt},
		metrics,
	)

	err := dispatcher.DispatchOnce(context.Background())

	require.ErrorContains(t, err, "stats query failed")
	require.Len(t, transport.published, 1)
	require.Equal(t, []string{record.ID}, store.published)
	require.Equal(t, float64(1), prometheusMetricValue(
		t,
		metrics.EventDispatcherError.WithLabelValues("stats"),
	))
}

func TestDispatcherTracksPublishFailureAndBackgroundLoopError(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	require.NoError(t, err)
	record := dispatcherRecord("event-1")
	store := &dispatcherStore{records: []persistence.OutboxRecord{record}}
	dispatcher := NewOutboxDispatcher(
		store,
		&fakeEventTransport{publishErr: errors.New("confirm failed")},
		OutboxDispatcherConfig{Owner: "worker-a", BatchSize: 1, Lease: time.Minute},
		fixedClock{now: record.OccurredAt},
		metrics,
	)
	require.Error(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventPublish.WithLabelValues("failure")))
	require.Zero(t, prometheusMetricValue(t, metrics.EventPublish.WithLabelValues("success")))
	require.Zero(t, prometheusMetricValue(
		t,
		metrics.EventDispatcherError.WithLabelValues("claim"),
	))

	store.claimErr = errors.New("database unavailable")
	dispatcher = NewOutboxDispatcher(
		store,
		&fakeEventTransport{},
		OutboxDispatcherConfig{
			Owner: "worker-b", BatchSize: 1, Lease: time.Minute, PollInterval: time.Millisecond,
		},
		fixedClock{now: record.OccurredAt},
		metrics,
	)
	require.NoError(t, dispatcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		return prometheusMetricValue(
			t,
			metrics.EventDispatcherError.WithLabelValues("claim"),
		) > 0
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Close())
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventPublish.WithLabelValues("failure")))
	require.Zero(t, prometheusMetricValue(t, metrics.EventPublish.WithLabelValues("success")))
	require.Zero(t, prometheusMetricValue(
		t,
		metrics.EventPublish.WithLabelValues("dispatcher_error"),
	))
}

func prometheusMetricValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()
	var value dto.Metric
	require.NoError(t, metric.Write(&value))
	if value.Counter != nil {
		return value.Counter.GetValue()
	}
	return value.Gauge.GetValue()
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func dispatcherRecord(id string) persistence.OutboxRecord {
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	return persistence.OutboxRecord{Envelope: events.Envelope{
		ID: id, Type: "test.event", Version: 1, OccurredAt: now,
	}, CreatedAt: now}
}

type blockingPublishTransport struct {
	fakeEventTransport
	started   chan struct{}
	release   chan struct{}
	cancelled chan struct{}
}

func (t *blockingPublishTransport) Publish(ctx context.Context, envelope events.Envelope) error {
	t.cancelled = make(chan struct{})
	close(t.started)
	select {
	case <-ctx.Done():
		close(t.cancelled)
		return ctx.Err()
	case <-t.release:
		t.published = append(t.published, envelope)
		return nil
	}
}
