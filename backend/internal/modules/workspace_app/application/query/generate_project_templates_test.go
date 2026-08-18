package query

import (
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestGenerateArtifacts_DefaultService(t *testing.T) {
	name, err := model.NewProjectName("test9")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "test9",
		Namespace:   "default",
		Language:    "java",
		GitURL:      "https://example.com/repo.git",
		GitRefType:  "branch",
		GitRefValue: "main",
		Resources: model.ProjectResources{
			ServiceTemplateID: "tmpl-svc",
			Replicas:           1,
		},
	})
	byID := map[string]*model.Template{
		"tmpl-svc": {
			ID:   kernel.TemplateID{},
			Name: mustTemplateName(t, "svc-base"),
			Kind: "Service",
		},
	}
	items := GenerateArtifacts(p, byID)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Kind != "Service" {
		t.Fatalf("kind=%s", items[0].Kind)
	}
	if !strings.Contains(items[0].Content, "name: test9-service") {
		t.Fatalf("content missing service name: %s", items[0].Content)
	}
	if !strings.Contains(items[0].Content, "namespace: default") {
		t.Fatalf("content missing namespace: %s", items[0].Content)
	}
}

func TestRenderPlaceholders(t *testing.T) {
	out := renderPlaceholders("hello {{name}} / ${namespace} / %GIT_URL%", map[string]string{
		"name":      "demo",
		"namespace": "ns1",
		"git_url":   "https://git",
	})
	want := "hello demo / ns1 / https://git"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func mustTemplateName(t *testing.T, raw string) model.TemplateName {
	t.Helper()
	n, err := model.NewTemplateName(raw)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
