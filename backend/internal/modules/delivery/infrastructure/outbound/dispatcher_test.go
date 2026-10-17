package outbound

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type recordingOperationWriter struct {
	requests []messaging.OperationRequest
}

func (w *recordingOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return nil
}

func TestPayloadFromBuildEvent(t *testing.T) {
	ev := deliveryevent.NewBuildCompleted(
		kernel.NewBuildID(), kernel.NewProjectID(), kernel.NewMemberID(), kernel.NewWorkspaceID(),
		"test-repo", "success", 3,
	)
	payload, name, projectID, ok := payloadFromEvent(ev)
	if !ok || name != "Build" || projectID.String() != ev.ProjectID.String() {
		t.Fatalf("unexpected payload mapping: ok=%v name=%s", ok, name)
	}
	if payload["project_name"] != "test-repo" {
		t.Fatalf("project name: %v", payload["project_name"])
	}
}

func TestPayloadFromRollbackEvent(t *testing.T) {
	ev := deliveryevent.NewDeployCompleted(
		kernel.NewDeploymentRunID(), kernel.NewProjectID(), kernel.NewWorkspaceID(), nil,
		"test-repo", "succeeded", "rollback", "v1",
	)
	_, name, _, ok := payloadFromEvent(ev)
	if !ok || name != "Rollback" {
		t.Fatalf("expected Rollback, got %s ok=%v", name, ok)
	}
}

func TestDispatcherPostsJSONAndToken(t *testing.T) {
	var gotToken string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-SparkOps-Token")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	d := &Dispatcher{client: server.Client(), sync: true}
	d.client.Timeout = 2 * time.Second
	d.dispatch(persistence.WebhookRecord{
		ID: kernel.NewWebhookID(), URL: server.URL, Token: "secret-token", Events: []string{"Build"},
	}, map[string]any{"event": "Build"})
	if gotToken != "secret-token" {
		t.Fatalf("token header: %s", gotToken)
	}
	if body["event"] != "Build" {
		t.Fatalf("body: %#v", body)
	}
}

func TestDispatcherSchedulesWebhookOperationWithoutHTTP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.project_webhooks (
		id TEXT PRIMARY KEY, project_id TEXT, name TEXT, url TEXT, token TEXT,
		events TEXT, enabled NUMERIC, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`).Error)
	projectID := kernel.NewProjectID()
	hook := &persistence.WebhookRecord{
		ID: kernel.NewWebhookID(), ProjectID: projectID, URL: "http://127.0.0.1:1/forbidden",
		Events: []string{"Build"}, Enabled: true,
	}
	require.NoError(t, persistence.NewWebhookRepository(db).Save(context.Background(), hook))
	writer := &recordingOperationWriter{}
	dispatcher := NewDispatcher(persistence.NewWebhookRepository(db), nil, writer)
	eventID := "event-webhook-1"
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: eventID, Subscriber: "delivery.build-completed.webhook",
	})

	err = dispatcher.Handle(ctx, deliveryevent.NewBuildCompleted(
		kernel.NewBuildID(), projectID, kernel.NewMemberID(), kernel.NewWorkspaceID(),
		"test-repo", "success", 3,
	))

	require.NoError(t, err)
	require.Len(t, writer.requests, 1)
	require.Equal(t, "delivery.webhook.send.v1", writer.requests[0].OperationType)
	require.Equal(t,
		eventID+":delivery.build-completed.webhook:delivery.webhook.send.v1:"+hook.ID.String(),
		writer.requests[0].IdempotencyKey,
	)
}
