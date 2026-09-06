package query

import (
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestBuiltinJenkinsfileTemplateDetailContainsPlaceholders(t *testing.T) {
	detail := BuiltinJenkinsfileTemplateDetail()
	for _, needle := range []string{
		"{{.name}}",
		"{{.git_credential_id}}",
		"{{.registry_credential_id}}",
		"params.BRANCH",
		"SparkOps CI",
		`{{if eq .enable_static_scan "true"}}`,
	} {
		if !strings.Contains(detail, needle) {
			t.Fatalf("builtin jenkinsfile missing %q", needle)
		}
	}
}

func TestGenerateArtifacts_UsesBuiltinJenkinsfileTemplateDetail(t *testing.T) {
	name, err := model.NewProjectName("orders")
	if err != nil {
		t.Fatal(err)
	}
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "orders",
		Namespace:   "team-a",
		Language:    "go",
		GitURL:      "git@gitlab.example.com:group/orders.git",
		GitRefType:  "branch",
		GitRefValue: "main",
		Resources: model.ProjectResources{
			JenkinsfileTemplateID: "jenkins-template",
			EnableStaticScan:      true,
		},
	})
	template := &model.Template{
		Kind:   "jenkinsFile",
		Name:   mustTemplateName(t, "jenkinsfile"),
		Detail: BuiltinJenkinsfileTemplateDetail(),
	}
	renderCtx := testRenderContext(project)
	renderCtx.GitCredentialID = "sparkops-ws-git"
	renderCtx.RegistryCredentialID = "sparkops-ws-harbor"

	items := GenerateArtifacts(
		project,
		map[string]*model.Template{"jenkins-template": template},
		renderCtx,
	)
	var pipeline string
	for _, item := range items {
		if item.Kind == "jenkinsFile" {
			pipeline = item.Content
			break
		}
	}
	for _, needle := range []string{
		"params.BRANCH",
		"sparkops-ws-git",
		"sparkops-ws-harbor",
		"SparkOps CI",
		"stage('Sonar Scan')",
	} {
		if !strings.Contains(pipeline, needle) {
			t.Fatalf("rendered pipeline missing %q: %s", needle, pipeline)
		}
	}
}

func TestIsLegacyJenkinsfileTemplateDetail(t *testing.T) {
	if !IsLegacyJenkinsfileTemplateDetail(`pipeline { agent any }`) {
		t.Fatal("expected legacy pipeline")
	}
	if IsLegacyJenkinsfileTemplateDetail(BuiltinJenkinsfileTemplateDetail()) {
		t.Fatal("expected builtin detail to be non-legacy")
	}
}
