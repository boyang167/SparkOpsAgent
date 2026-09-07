package query

import (
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestInjectProjectConfigEnv_AppendsBeforeVolumeMounts(t *testing.T) {
	source := `      containers:
      - image: demo:latest
        name: demo
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        volumeMounts:
        - name: tz-config
          mountPath: /etc/localtime`

	out := injectProjectConfigEnv(source, `{"APP_ENV":"prod","LOG_LEVEL":"debug"}`)
	if strings.Contains(out, "<no value>") {
		t.Fatalf("unexpected placeholder: %s", out)
	}
	if !strings.Contains(out, "- name: APP_ENV\n          value: prod") {
		t.Fatalf("missing APP_ENV: %s", out)
	}
	if !strings.Contains(out, "- name: LOG_LEVEL\n          value: debug") {
		t.Fatalf("missing LOG_LEVEL: %s", out)
	}
	if !strings.Contains(out, "MY_POD_NAME") {
		t.Fatalf("removed existing env: %s", out)
	}
}

func TestInjectProjectConfigEnv_CreatesEnvBlockWhenMissing(t *testing.T) {
	source := `      containers:
      - image: demo:latest
        name: demo
        resources:
          requests:
            memory: 512Mi`

	out := injectProjectConfigEnv(source, `{"DB_HOST":"postgres"}`)
	if !strings.Contains(out, "env:\n        - name: DB_HOST") {
		t.Fatalf("missing env block: %s", out)
	}
}

func TestInjectProjectConfigEnv_AppendsWhenEnvFromPrecedesEnv(t *testing.T) {
	source := `      containers:
      - image: demo:latest
        name: demo
        resources:
          requests:
            memory: 512Mi
          limits:
            memory: 1Gi
        envFrom:
        - configMapRef:
            name: bionet-env-config
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        volumeMounts:
        - name: tz-config
          mountPath: /etc/localtime`

	out := injectProjectConfigEnv(source, `{"env":"dev"}`)
	if strings.Contains(out, "limits:\n            memory: 1Gi\n        - name: env") {
		t.Fatalf("env item injected at wrong level: %s", out)
	}
	if !strings.Contains(out, "- name: env\n          value: dev") {
		t.Fatalf("missing env var: %s", out)
	}
	if !strings.Contains(out, "MY_POD_NAME") {
		t.Fatalf("removed existing env: %s", out)
	}
}

func TestGenerateArtifacts_InjectsProjectConfigEnv(t *testing.T) {
	name, err := model.NewProjectName("demo")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:      "demo",
		Namespace: "test-workspace",
		Resources: model.ProjectResources{
			DeploymentTemplateID: "dep-1",
			ProjectConfig:        `{"APP_MODE":"production"}`,
		},
	})
	templateName, err := model.NewTemplateName("dep")
	if err != nil {
		t.Fatal(err)
	}
	template := model.NewTemplate(
		kernel.NewWorkspaceID(),
		templateName,
		"Deployment",
		"go",
		`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.name}}
spec:
  template:
    spec:
      containers:
      - image: {{.image}}
        name: {{.name}}
        volumeMounts:
        - name: tz-config
          mountPath: /etc/localtime`,
		"",
		model.TemplateStatusActive,
		false,
		kernel.NewMemberID(),
	)

	byID := map[string]*model.Template{"dep-1": template}
	renderCtx := testRenderContext(p)
	items := GenerateArtifacts(p, byID, renderCtx)
	var content string
	for _, item := range items {
		if item.Kind == "Deployment" {
			content = item.Content
			break
		}
	}
	if content == "" {
		t.Fatalf("missing deployment artifact: %+v", items)
	}
	if !strings.Contains(content, "- name: APP_MODE\n          value: production") {
		t.Fatalf("project config not injected: %s", content)
	}
}
