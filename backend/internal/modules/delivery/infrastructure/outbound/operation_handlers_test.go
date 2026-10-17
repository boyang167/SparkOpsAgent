package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
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
