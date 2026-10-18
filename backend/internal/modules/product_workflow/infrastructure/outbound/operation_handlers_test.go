package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/stretchr/testify/require"
)

type staticWebhookResolver struct {
	url  string
	body json.RawMessage
}

func (r staticWebhookResolver) ResolveWebhook(context.Context, NotificationPayload) (string, json.RawMessage, bool, error) {
	return r.url, r.body, true, nil
}

func TestWebhookHandlerTreatsRequestTimeoutAsTemporaryAndForwardsIdempotency(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-SparkOps-Idempotency-Key")
		w.WriteHeader(http.StatusRequestTimeout)
	}))
	defer server.Close()
	payload, err := json.Marshal(NotificationPayload{EventID: "event-1", NoteID: "hook-a"})
	if err != nil {
		t.Fatal(err)
	}

	err = NewWebhookHandler(staticWebhookResolver{url: server.URL, body: json.RawMessage(`{}`)}, server.Client())(context.Background(), messaging.Operation{
		Payload: payload, IdempotencyKey: "event-1:subscriber:type:target",
	})
	if err == nil {
		t.Fatal("expected temporary error")
	}
	if messaging.IsPermanent(err) {
		t.Fatalf("408 must be temporary: %v", err)
	}
	if gotKey != "event-1:subscriber:type:target" {
		t.Fatalf("idempotency key = %q", gotKey)
	}
}

func TestWebhookHandlerSanitizesURLFromNetworkError(t *testing.T) {
	const secret = "workflow-secret-token"
	const endpoint = "https://127.0.0.1:1/hook?access_token=" + secret
	payload, err := json.Marshal(NotificationPayload{
		EventID: "event-1", NoteID: "hook-a", Timeout: time.Second,
	})
	require.NoError(t, err)
	client := &http.Client{Transport: workflowRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}

	err = NewWebhookHandler(
		staticWebhookResolver{url: endpoint, body: json.RawMessage(`{}`)},
		client,
	)(context.Background(), messaging.Operation{Payload: payload})

	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), endpoint)
}

func TestWebhookHandlerSanitizesMalformedURL(t *testing.T) {
	const secret = "workflow-build-secret"
	const endpoint = "https://example.com/%zz?access_token=" + secret
	payload, err := json.Marshal(NotificationPayload{
		EventID: "event-1", NoteID: "hook-a",
	})
	require.NoError(t, err)

	err = NewWebhookHandler(
		staticWebhookResolver{url: endpoint, body: json.RawMessage(`{}`)},
		http.DefaultClient,
	)(context.Background(), messaging.Operation{Payload: payload})

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), endpoint)
}

type workflowRoundTripFunc func(*http.Request) (*http.Response, error)

func (f workflowRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNotificationHandlerPassesIdempotencyKeyToSender(t *testing.T) {
	payload, err := json.Marshal(NotificationPayload{Target: "mail-a"})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	handler := NewNotificationHandler(func(_ context.Context, payload NotificationPayload) error {
		got = payload.IdempotencyKey
		return nil
	})
	if err := handler(context.Background(), messaging.Operation{
		Payload: payload, IdempotencyKey: "event-1:subscriber:type:mail-a",
	}); err != nil {
		t.Fatal(err)
	}
	if got != "event-1:subscriber:type:mail-a" {
		t.Fatalf("idempotency key = %q", got)
	}
}
