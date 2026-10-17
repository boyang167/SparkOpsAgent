package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

func TestWebhookHandlerTreatsRequestTimeoutAsTemporaryAndForwardsIdempotency(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-SparkOps-Idempotency-Key")
		w.WriteHeader(http.StatusRequestTimeout)
	}))
	defer server.Close()
	payload, err := json.Marshal(WebhookPayload{EventID: "event-1", URL: server.URL, Body: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}

	err = NewWebhookHandler(server.Client())(context.Background(), messaging.Operation{
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
