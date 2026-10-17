package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/stretchr/testify/require"
)

func TestWebhookOperationForwardsStableIdempotencyHeaders(t *testing.T) {
	var eventID, idempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventID = r.Header.Get("X-SparkOps-Event-ID")
		idempotencyKey = r.Header.Get("X-SparkOps-Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	payload, err := json.Marshal(WebhookPayload{
		EventID: "event-1", WebhookID: "hook-1", URL: server.URL,
		Body: map[string]any{"event": "Build"},
	})
	require.NoError(t, err)

	err = NewWebhookHandler(server.Client()).Handle(context.Background(), messaging.Operation{
		IdempotencyKey: "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:hook-1",
		Payload:        payload,
	})

	require.NoError(t, err)
	require.Equal(t, "event-1", eventID)
	require.Equal(t, "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:hook-1", idempotencyKey)
}

func TestWebhookOperationClassifiesPermanentRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	payload, err := json.Marshal(WebhookPayload{EventID: "event-1", URL: server.URL, Body: map[string]any{}})
	require.NoError(t, err)

	err = NewWebhookHandler(server.Client()).Handle(context.Background(), messaging.Operation{
		IdempotencyKey: "event-1:owner:delivery.webhook.send.v1:hook-1", Payload: payload,
	})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}
