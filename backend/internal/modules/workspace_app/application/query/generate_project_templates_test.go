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
			Replicas:          1,
		},
	})
	byID := map[string]*model.Template{
		"tmpl-svc": {
			ID:   kernel.TemplateID{},
			Name: mustTemplateName(t, "svc-base"),
			Kind: "Service",
		},
	}
	items := GenerateArtifacts(p, byID, testRenderContext(p))
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

func TestGenerateArtifacts_IncludesServiceWhenDeploymentConfigured(t *testing.T) {
	name, err := model.NewProjectName("demo")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:      "demo",
		Namespace: "default",
		Resources: model.ProjectResources{
			DeploymentTemplateID:  "tmpl-dep",
			JenkinsfileTemplateID: "tmpl-jen",
			Replicas:              1,
		},
	})
	byID := map[string]*model.Template{
		"tmpl-dep": {Kind: "Deployment"},
		"tmpl-jen": {Kind: "jenkinsFile"},
	}
	items := GenerateArtifacts(p, byID, testRenderContext(p))
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	if items[0].Kind != "Service" {
		t.Fatalf("first kind=%s want Service", items[0].Kind)
	}
	if !strings.Contains(items[0].Content, "kind: Service") {
		t.Fatalf("service content missing: %s", items[0].Content)
	}
}

func TestRenderDeploymentPipelineArtifactUsesDeploymentOverrides(t *testing.T) {
	name, err := model.NewProjectName("orders")
	if err != nil {
		t.Fatal(err)
	}
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "orders",
		Namespace:   "team-a",
		GitRefValue: "stored-branch",
		Resources: model.ProjectResources{
			JenkinsfileTemplateID: "jenkins-template",
		},
	})
	template := &model.Template{
		Kind: "jenkinsFile",
		Name: mustTemplateName(t, "deploy-pipeline"),
		Detail: `pipeline {
  environment {
    REF = '{{ .CodeVersion }}'
    IMAGE = '{{ .image }}'
    GIT_CREDENTIAL = '{{ .git_credential_id }}'
    REGISTRY_CREDENTIAL = '{{ .registry_credential_id }}'
  }
}`,
	}

	content := RenderDeploymentPipelineArtifact(
		project,
		map[string]*model.Template{"jenkins-template": template},
		testRenderContext(project),
		DeploymentPipelineRenderInput{
			GitRefValue:          "release/2026-08",
			ImageTag:             "deploy-123",
			ImageRepository:      "registry.example/team-a/orders",
			GitCredentialID:      "sparkops-ws-git",
			RegistryCredentialID: "sparkops-ws-harbor",
		},
	)

	if !strings.Contains(content, "release/2026-08") {
		t.Fatalf("rendered pipeline missing deployment git ref: %s", content)
	}
	if !strings.Contains(content, "registry.example/team-a/orders:deploy-123") {
		t.Fatalf("rendered pipeline missing deployment image: %s", content)
	}
}

func TestRenderDeploymentPipelineArtifactUsesDeploymentGitRefType(t *testing.T) {
	name, err := model.NewProjectName("orders")
	if err != nil {
		t.Fatal(err)
	}
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "orders",
		Namespace:   "team-a",
		GitRefType:  "branch",
		GitRefValue: "main",
		Resources: model.ProjectResources{
			JenkinsfileTemplateID: "jenkins-template",
		},
	})
	template := &model.Template{
		Kind: "jenkinsFile",
		Name: mustTemplateName(t, "deploy-pipeline"),
		Detail: `pipeline {
  environment {
    GIT_REF_TYPE = '{{ .git_ref_type }}'
  }
}`,
	}

	content := RenderDeploymentPipelineArtifact(
		project,
		map[string]*model.Template{"jenkins-template": template},
		testRenderContext(project),
		DeploymentPipelineRenderInput{
			GitRefType:      "tag",
			GitRefValue:     "v1.0.1",
			ImageTag:        "v1.0.1",
			ImageRepository: "registry.example/team-a/orders",
		},
	)

	if !strings.Contains(content, "GIT_REF_TYPE = 'tag'") {
		t.Fatalf("rendered pipeline missing deployment git ref type: %s", content)
	}
}

func TestDefaultJenkinsfileIncludesSparkOpsRuntimeParams(t *testing.T) {
	name, err := model.NewProjectName("orders")
	if err != nil {
		t.Fatal(err)
	}
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:           "orders",
		Namespace:      "team-a",
		Language:       "go",
		GitURL:         "git@gitlab.example.com:group/orders.git",
		GitRefType:     "branch",
		GitRefValue:    "main",
		DockerfilePath: "./Dockerfile",
		Resources: model.ProjectResources{
			JenkinsfileTemplateID: "jenkins-template",
			RegistryType:          "Harbor",
			EnableStaticScan:      true,
		},
	})
	renderCtx := testRenderContext(project)
	renderCtx.GitCredentialID = "sparkops-ws-git"
	renderCtx.RegistryCredentialID = "sparkops-ws-harbor"
	vars := buildTemplateVars(project, renderCtx)
	content := defaultJenkinsfile(vars, project.Resources.EnableStaticScan)

	for _, needle := range []string{
		"params.BRANCH",
		"params.IMAGE_TAG",
		"params.IMAGE_REPOSITORY",
		"params.CodeVersion",
		"params.ROLLBACK_OF",
		"sparkops-ws-git",
		"sparkops-ws-harbor",
		"git@gitlab.example.com:group/orders.git",
		"stage('Sonar Scan')",
		"docker push",
		"certs.d",
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("jenkinsfile missing %q: %s", needle, content)
		}
	}
	for _, forbidden := range []string{
		"Prepare Registry",
		"/api/v2.0/projects/",
		"Kaniko",
		"kaniko",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("jenkinsfile should not contain %q", forbidden)
		}
	}
}

func TestDefaultJenkinsfilePreservesGroovyEnvReferences(t *testing.T) {
	vars := map[string]string{
		"name":               "test-go",
		"namespace":          "test-workspace",
		"language":           "go",
		"git_url":            "http://192.168.5.24:8929/cicd/test-repo.git",
		"git_ref_type":       "branch",
		"git_ref_value":      "main",
		"workspace_slug":     "test-workspace",
		"registry_host":      "harbor.local",
		"git_credential_id":  "sparkops-git-test-workspace-abc",
		"registry_credential_id": "sparkops-harbor-test-workspace-abc",
	}
	content := defaultJenkinsfile(vars, false)

	for _, needle := range []string{
		`APP_NAME               = 'test-go'`,
		`DEFAULT_GIT_REF        = 'main'`,
		`${env.APP_NAME}`,
		`${env.BRANCH}`,
		`${env.FULL_IMAGE}`,
		`?: 'main'`,
		`def branch = (env.BRANCH ?: 'main').trim()`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("jenkinsfile missing %q:\n%s", needle, content)
		}
	}
}

func TestRenderPlaceholdersPreservesUnknownDollarTokens(t *testing.T) {
	out := renderPlaceholders("keep ${env.BRANCH} replace ${namespace}", map[string]string{
		"namespace": "ns1",
	})
	want := "keep ${env.BRANCH} replace ns1"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
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

func TestRenderTemplateSource_GoTemplatePorts(t *testing.T) {
	name, err := model.NewProjectName("test-project")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:      "test-project",
		Namespace: "test-workspace",
		Resources: model.ProjectResources{
			Ports: []model.ProjectPort{
				{Name: "http-80", ContainerPort: 80, ServicePort: 80, Protocol: "TCP"},
			},
		},
	})

	source := `apiVersion: v1
kind: Service
metadata:
  name: {{.name}}
  namespace: {{.namespace}}
spec:
  ports:
{{- range $k,$v := .ports }}
    - port: {{$v.port}}
      targetPort: {{$v.targetPort}}
      name: {{$v.name}}
      protocol: {{$v.protocol}}
{{- end }}
  {{if eq .resourceType "service"}}selector:
    app: {{.name}}
{{end}}`

	out := renderTemplateSource(source, p, "Service", testRenderContext(p))
	if strings.Contains(out, "{{range") || strings.Contains(out, "{{$v.port}}") {
		t.Fatalf("ports not rendered: %s", out)
	}
	if !strings.Contains(out, "port: 80") || !strings.Contains(out, "name: http-80") {
		t.Fatalf("missing port fields: %s", out)
	}
	if !strings.Contains(out, "selector:") || !strings.Contains(out, "app: test-project") {
		t.Fatalf("missing selector: %s", out)
	}
}

func TestRenderTemplateSource_PortRangeFallback(t *testing.T) {
	name, err := model.NewProjectName("demo")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name: "demo",
		Resources: model.ProjectResources{
			Ports: []model.ProjectPort{{Name: "http-8080", ContainerPort: 8080, Protocol: "TCP"}},
		},
	})

	// Missing {{end}} — Go template parse fails, fallback expands range block.
	source := `spec:
  ports:
{{range $k,$v := .ports}}
    - port: {{$v.port}}
      name: {{$v.name}}
      protocol: {{$v.protocol}}
{{if eq .resourceType "service"}}selector:
    app: demo`

	out := renderTemplateSource(source, p, "Service", testRenderContext(p))
	if strings.Contains(out, "{{$v.port}}") {
		t.Fatalf("ports not expanded: %s", out)
	}
	if !strings.Contains(out, "port: 8080") {
		t.Fatalf("missing port: %s", out)
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

func testRenderContext(project *model.Project) TemplateRenderContext {
	ws := sanitizeTemplateSegment(project.Namespace)
	if ws == "" {
		ws = "default"
	}
	gitRef := orDefault(project.GitRefValue, "latest")
	projectName := project.Name.String()
	ctx := TemplateRenderContext{
		OrgName:            "test-org",
		OrgSlug:            "test-org",
		WorkspaceName:      ws,
		WorkspaceSlug:      ws,
		RegistrySecretName: "harbor",
		RegistryURL:        "https://192.168.5.24",
		RegistryHost:       "192.168.5.24",
	}
	ctx.ContainerImage = "192.168.5.24/" + ws + "/" + projectName + ":" + gitRef
	ctx.ImagePullSecret = buildImagePullSecretName(ctx.OrgSlug, ctx.WorkspaceSlug, ctx.RegistrySecretName, projectName)
	return ctx
}

func TestRenderTemplateSource_DeploymentVars(t *testing.T) {
	name, err := model.NewProjectName("test-project")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "test-project",
		Namespace:   "test-workspace",
		GitRefValue: "master",
		Resources: model.ProjectResources{
			MemoryRequest:    "512Mi",
			MemoryLimit:      "1Gi",
			RegistrySecretID: "secret-1",
		},
	})
	renderCtx := testRenderContext(p)
	renderCtx.RegistrySecretName = "harbor"
	renderCtx.ImagePullSecret = buildImagePullSecretName("test-org", "test-workspace", "harbor", "test-project")

	source := `imagePullSecrets:
  - name: {{ .imagePullSecretName }}
containers:
  - image: {{ .image }}
resources:
  requests:
    memory: {{ .resources.requests.memory }}
  limits:
    memory: {{ .resources.limits.memory }}`

	out := renderTemplateSource(source, p, "Deployment", renderCtx)
	if strings.Contains(out, "<no value>") || strings.Contains(out, "{{") {
		t.Fatalf("unrendered template: %s", out)
	}
	if !strings.Contains(out, "512Mi") || !strings.Contains(out, "1Gi") {
		t.Fatalf("missing memory resources: %s", out)
	}
	if !strings.Contains(out, "192.168.5.24/test-workspace/test-project:master") {
		t.Fatalf("missing image ref: %s", out)
	}
	if !strings.Contains(out, "test-org-test-workspace-harbor-test-project-") {
		t.Fatalf("missing pull secret prefix: %s", out)
	}
}

func TestRenderTemplateSource_LegacyDeploymentTemplate(t *testing.T) {
	name, err := model.NewProjectName("test-project4")
	if err != nil {
		t.Fatal(err)
	}
	p := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{
		Name:        "test-project4",
		Namespace:   "test-workspace",
		GitRefValue: "master",
		Resources: model.ProjectResources{
			MemoryRequest: "512Mi",
			MemoryLimit:   "1Gi",
			CPURequest:    "500m",
			CPULimit:      "1000m",
			Ports: []model.ProjectPort{
				{Name: "http-8", ContainerPort: 8, Protocol: "TCP"},
			},
		},
	})
	renderCtx := testRenderContext(p)
	renderCtx.RegistrySecretName = "harbor"
	renderCtx.ImagePullSecret = buildImagePullSecretName("test-org", "test-workspace", "harbor", "test-project4")

	source := `imagePullSecrets:
      - name:  {{.imagesecret}}
      containers:
      - image: {{.image}}
        name: {{.name}}
        ports:
        - containerPort: {{index .ports 0 "port"}}
        resources:
          requests:
            memory: {{.memory}}
          limits:
            memory: {{.maxMemory}}`

	out := renderTemplateSource(source, p, "Deployment", renderCtx)
	if strings.Contains(out, "<no value>") || strings.Contains(out, "{{") {
		t.Fatalf("unrendered legacy template: %s", out)
	}
	if !strings.Contains(out, "512Mi") || !strings.Contains(out, "1Gi") {
		t.Fatalf("missing memory: %s", out)
	}
	if !strings.Contains(out, "192.168.5.24/test-workspace/test-project4:master") {
		t.Fatalf("missing image: %s", out)
	}
	if !strings.Contains(out, "test-org-test-workspace-harbor-test-project4-") {
		t.Fatalf("missing imagesecret: %s", out)
	}
	if !strings.Contains(out, "containerPort: 8") {
		t.Fatalf("missing port: %s", out)
	}
}
