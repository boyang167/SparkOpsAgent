package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type webhookLookupStub struct {
	hook *persistence.WebhookRecord
	err  error
}

func (s webhookLookupStub) Get(context.Context, kernel.WebhookID) (*persistence.WebhookRecord, error) {
	return s.hook, s.err
}

func TestWebhookOperationForwardsStableIdempotencyHeaders(t *testing.T) {
	var eventID, idempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventID = r.Header.Get("X-SparkOps-Event-ID")
		idempotencyKey = r.Header.Get("X-SparkOps-Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{
		EventID: "event-1", WebhookID: hookID.String(),
		Body: map[string]any{"event": "Build"},
	})
	require.NoError(t, err)

	err = NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: server.URL, Token: "latest-token", Enabled: true,
	}}, server.Client()).Handle(context.Background(), messaging.Operation{
		IdempotencyKey: "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:" + hookID.String(),
		Payload:        payload,
	})

	require.NoError(t, err)
	require.Equal(t, "event-1", eventID)
	require.Equal(t, "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:"+hookID.String(), idempotencyKey)
}

func TestWebhookOperationClassifiesPermanentRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{EventID: "event-1", WebhookID: hookID.String(), Body: map[string]any{}})
	require.NoError(t, err)

	err = NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: server.URL, Enabled: true,
	}}, server.Client()).Handle(context.Background(), messaging.Operation{
		IdempotencyKey: "event-1:owner:delivery.webhook.send.v1:hook-1", Payload: payload,
	})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}

func TestWebhookOperationTreatsRequestTimeoutAsTemporary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusRequestTimeout)
	}))
	defer server.Close()
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{EventID: "event-1", WebhookID: hookID.String(), Body: map[string]any{}})
	require.NoError(t, err)

	err = NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: server.URL, Enabled: true,
	}}, server.Client()).Handle(context.Background(), messaging.Operation{Payload: payload})

	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
}

func TestWebhookOperationSanitizesURLFromNetworkError(t *testing.T) {
	const secret = "delivery-secret-token"
	const endpoint = "https://127.0.0.1:1/hook?token=" + secret
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{
		EventID: "event-1", WebhookID: hookID.String(), Body: map[string]any{},
		Timeout: time.Second,
	})
	require.NoError(t, err)
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}

	err = NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: endpoint, Enabled: true,
	}}, client).Handle(context.Background(), messaging.Operation{Payload: payload})

	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), endpoint)
}

func TestWebhookOperationSanitizesControlCharacterURL(t *testing.T) {
	const secret = "operation-build-secret"
	const endpoint = "https://example.com/hook?token=" + secret + "\n"
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{
		EventID: "event-1", WebhookID: hookID.String(), Body: map[string]any{},
	})
	require.NoError(t, err)

	err = NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: endpoint, Enabled: true,
	}}, http.DefaultClient).Handle(context.Background(), messaging.Operation{Payload: payload})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), endpoint)
}

func TestWebhookOperationPersistsSanitizedLastError(t *testing.T) {
	const secret = "persisted-secret-token"
	const endpoint = "https://127.0.0.1:1/hook?token=" + secret
	hookID := kernel.NewWebhookID()
	payload, err := json.Marshal(WebhookPayload{
		EventID: "event-1", WebhookID: hookID.String(), Body: map[string]any{},
	})
	require.NoError(t, err)
	handler := NewWebhookHandler(webhookLookupStub{hook: &persistence.WebhookRecord{
		ID: hookID, URL: endpoint, Enabled: true,
	}}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})})
	operation := messaging.Operation{ID: "operation-1", Payload: payload}
	handlerErr := handler.Handle(context.Background(), operation)
	require.Error(t, handlerErr)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform.operation_outbox (
		id TEXT PRIMARY KEY, status TEXT, locked_by TEXT, locked_at DATETIME,
		attempts INTEGER DEFAULT 1, next_attempt_at DATETIME, last_error TEXT,
		completed_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO platform.operation_outbox(id, status, locked_by) VALUES (?, 'executing', 'worker-a')",
		operation.ID,
	).Error)
	store := messagingpersistence.NewOperationStore(db)
	require.NoError(t, store.Reschedule(
		context.Background(), operation.ID, "worker-a", handlerErr.Error(), time.Now(),
	))
	var lastError string
	require.NoError(t, db.Raw(
		"SELECT last_error FROM platform.operation_outbox WHERE id = ?", operation.ID,
	).Scan(&lastError).Error)
	require.NotContains(t, lastError, secret)
	require.NotContains(t, lastError, endpoint)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
