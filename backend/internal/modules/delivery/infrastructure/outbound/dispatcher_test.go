package outbound

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

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
