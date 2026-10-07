package query

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const defaultTemplatesDir = "../command/default_templates"

var kindLineRe = regexp.MustCompile(`(?m)^kind:\s*(\S+)`)

func TestDefaultTemplatesRenderToValidManifests(t *testing.T) {
	entries, err := os.ReadDir(defaultTemplatesDir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		label string
		res   model.ProjectResources
	}{
		{label: "bare"},
		{label: "full", res: model.ProjectResources{
			Replicas:      2,
			Command:       "/app/server\n--config=/etc/app.yaml",
			Args:          "--verbose",
			ProjectConfig: "LOG_LEVEL=info\ningress_host=demo.example.org\nstorage_class=nfs\nstorage_size=20Gi",
			Ports: []model.ProjectPort{
				{Name: "http", ContainerPort: 8080, ServicePort: 80, Protocol: "TCP"},
				{Name: "grpc", ContainerPort: 9090, Protocol: "TCP"},
			},
		}},
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(defaultTemplatesDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		match := kindLineRe.FindStringSubmatch(string(raw))
		if match == nil {
			t.Fatalf("%s: cannot detect kind", entry.Name())
		}
		kind := match[1]
		for _, c := range cases {
			name, _ := model.NewProjectName("demo")
			p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
				Name: "demo", Namespace: "test-workspace", GitRefValue: "main", Resources: c.res,
			})
			ctx := testRenderContext(p)
			if _, err := renderGoTemplate(string(raw), buildTemplateData(p, kind, ctx)); err != nil {
				t.Fatalf("%s/%s: template error: %v", entry.Name(), c.label, err)
			}
			out := renderTemplateSource(string(raw), p, kind, ctx)
			if kind == "Deployment" {
				out = injectProjectConfigEnv(out, p.Resources.ProjectConfig)
			}
			if strings.Contains(out, "<no value>") || strings.Contains(out, "{{") {
				t.Fatalf("%s/%s: unrendered output:\n%s", entry.Name(), c.label, out)
			}
			var doc map[string]any
			if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("%s/%s: invalid yaml: %v\n%s", entry.Name(), c.label, err, out)
			}
			if doc["kind"] != kind {
				t.Fatalf("%s/%s: kind %v, want %s", entry.Name(), c.label, doc["kind"], kind)
			}
		}
	}
}
