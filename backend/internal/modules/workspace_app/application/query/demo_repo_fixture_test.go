package query

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func demoRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "demo", "test-repo"))
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Skipf("demo repo fixture missing: %v", err)
	}
	return root
}

func TestDemoRepoDockerfileIsRunnable(t *testing.T) {
	root := demoRepoRoot(t)
	for _, name := range []string{"Dockerfile", filepath.Join("docker", "Dockerfile")} {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(content)
		for _, want := range []string{"EXPOSE 8080", "HEALTHCHECK", "/health", "go build", "LISTEN_ADDR=:8080"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", name, want)
			}
		}
	}
}

func TestDemoRepoUnitTestsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping demo repo go test in short mode")
	}
	root := demoRepoRoot(t)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demo repo tests failed: %v\n%s", err, out)
	}
}

func TestGenerateArtifactsForDemoGitRepo(t *testing.T) {
	gitURL := "http://192.168.5.24:8929/cicd/test-repo.git"
	name, err := model.NewProjectName("test-repo")
	if err != nil {
		t.Fatal(err)
	}
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:           "test-repo",
		Namespace:      "demo-ws",
		Language:       "go",
		GitURL:         gitURL,
		GitRefType:     "branch",
		GitRefValue:    "master",
		DockerfilePath: "./Dockerfile",
		Resources: model.ProjectResources{
			JenkinsfileTemplateID: "jenkins-template",
			RegistryType:          "Harbor",
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

	var pipeline string
	for _, item := range GenerateArtifacts(project, map[string]*model.Template{"jenkins-template": template}, renderCtx) {
		if item.Kind == "jenkinsFile" {
			pipeline = item.Content
			break
		}
	}
	for _, needle := range []string{
		gitURL,
		"./Dockerfile",
		"DOCKERFILE_PATH",
		"docker build",
		"docker push",
		"SparkOps CI",
	} {
		if !strings.Contains(pipeline, needle) {
			t.Fatalf("rendered pipeline missing %q", needle)
		}
	}
}
