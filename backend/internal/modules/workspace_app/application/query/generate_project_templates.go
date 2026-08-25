package query

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GeneratedTemplateArtifact struct {
	Key          string `json:"key"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Filename     string `json:"filename"`
	Content      string `json:"content"`
	TemplateID   string `json:"template_id,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
}

type GenerateProjectTemplatesHandler struct {
	projectRepo  repository.ProjectRepository
	templateRepo repository.TemplateRepository
	wsChecker    port.WorkspaceChecker
}

func NewGenerateProjectTemplatesHandler(
	projectRepo repository.ProjectRepository,
	templateRepo repository.TemplateRepository,
	wsChecker port.WorkspaceChecker,
) *GenerateProjectTemplatesHandler {
	return &GenerateProjectTemplatesHandler{
		projectRepo:  projectRepo,
		templateRepo: templateRepo,
		wsChecker:    wsChecker,
	}
}

func (h *GenerateProjectTemplatesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) ([]GeneratedTemplateArtifact, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}

	project, err := h.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID || project.Status != model.ProjectStatusActive {
		return nil, domain.ErrProjectNotFound
	}

	allTemplates, err := ListAccessibleTemplates(ctx, h.templateRepo, h.wsChecker, wsID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Template, len(allTemplates))
	for _, t := range allTemplates {
		byID[t.ID.String()] = t
	}

	return GenerateArtifacts(project, byID), nil
}

type genPick struct {
	kind       string
	templateID string
}

func GenerateArtifacts(project *model.Project, byID map[string]*model.Template) []GeneratedTemplateArtifact {
	vars := buildTemplateVars(project)
	picks := make([]genPick, 0, 3)

	if id := strings.TrimSpace(project.Resources.ServiceTemplateID); id != "" {
		picks = append(picks, genPick{kind: "Service", templateID: id})
	}
	if id := strings.TrimSpace(project.Resources.JenkinsfileTemplateID); id != "" {
		picks = append(picks, genPick{kind: "jenkinsFile", templateID: id})
	}
	if id := strings.TrimSpace(project.Resources.DeploymentTemplateID); id != "" {
		picks = append(picks, genPick{kind: "Deployment", templateID: id})
	}
	if len(picks) == 0 && strings.TrimSpace(project.TemplateID) != "" {
		kind := "Deployment"
		if t := byID[project.TemplateID]; t != nil {
			if n := normalizeKind(t.Kind); n != "" {
				kind = n
			}
		}
		picks = append(picks, genPick{kind: kind, templateID: project.TemplateID})
	}

	order := map[string]int{"Service": 0, "jenkinsFile": 1, "Deployment": 2}
	sort.SliceStable(picks, func(i, j int) bool {
		return order[picks[i].kind] < order[picks[j].kind]
	})

	out := make([]GeneratedTemplateArtifact, 0, len(picks))
	for _, pick := range picks {
		var template *model.Template
		if pick.templateID != "" {
			template = byID[pick.templateID]
		}
		source := ""
		templateName := ""
		if template != nil {
			source = strings.TrimSpace(template.Detail)
			templateName = template.Name.String()
		}
		content := ""
		if source != "" && looksLikeManifest(source) {
			content = renderPlaceholders(source, vars)
		} else {
			content = defaultContent(pick.kind, project, vars)
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		title := pick.kind
		if pick.kind == "jenkinsFile" {
			title = "Jenkinsfile"
		}
		out = append(out, GeneratedTemplateArtifact{
			Key:          fmt.Sprintf("%s-%s", pick.kind, orDefault(pick.templateID, "default")),
			Kind:         pick.kind,
			Title:        title,
			Filename:     filenameFor(pick.kind, vars["name"]),
			Content:      content,
			TemplateID:   pick.templateID,
			TemplateName: templateName,
		})
	}
	return out
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "service":
		return "Service"
	case "deployment":
		return "Deployment"
	case "jenkinsfile":
		return "jenkinsFile"
	default:
		return ""
	}
}

func looksLikeManifest(text string) bool {
	sample := strings.ToLower(text)
	if len(sample) > 400 {
		sample = sample[:400]
	}
	return strings.Contains(sample, "apiversion:") ||
		strings.Contains(sample, "kind:") ||
		strings.Contains(sample, "pipeline {") ||
		strings.Contains(sample, "pipeline{") ||
		strings.Contains(sample, "#!groovy") ||
		strings.Contains(sample, "stages {")
}

var (
	mustacheRe = regexp.MustCompile(`\{\{\s*\.?([a-zA-Z0-9_.-]+)\s*\}\}`)
	dollarRe   = regexp.MustCompile(`\$\{\s*([a-zA-Z0-9_.-]+)\s*\}`)
	percentRe  = regexp.MustCompile(`%([A-Z0-9_]+)%`)
)

func renderPlaceholders(source string, vars map[string]string) string {
	out := mustacheRe.ReplaceAllStringFunc(source, func(m string) string {
		sub := mustacheRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		return lookupVar(vars, sub[1])
	})
	out = dollarRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := dollarRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		return lookupVar(vars, sub[1])
	})
	out = percentRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := percentRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		return lookupVar(vars, strings.ToLower(sub[1]))
	})
	return out
}

func lookupVar(vars map[string]string, key string) string {
	if v, ok := vars[key]; ok {
		return v
	}
	if v, ok := vars[strings.ToLower(key)]; ok {
		return v
	}
	return ""
}

func buildTemplateVars(project *model.Project) map[string]string {
	name := project.Name.String()
	if name == "" {
		name = "app"
	}
	ns := strings.TrimSpace(project.Namespace)
	if ns == "" {
		ns = "default"
	}
	display := strings.TrimSpace(project.DisplayName)
	if display == "" {
		display = name
	}
	dockerfile := strings.TrimSpace(project.DockerfilePath)
	if dockerfile == "" {
		dockerfile = "./Dockerfile"
	}
	res := project.Resources
	cpuReq := orDefault(res.CPURequest, "500m")
	cpuLim := orDefault(res.CPULimit, "1000m")
	memReq := orDefault(res.MemoryRequest, "512Mi")
	memLim := orDefault(res.MemoryLimit, "1Gi")
	replicas := res.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	registry := orDefault(res.RegistryType, "Harbor")
	gitRefType := orDefault(project.GitRefType, "branch")
	gitRefValue := orDefault(project.GitRefValue, "main")

	vars := map[string]string{
		"name":            name,
		"project_name":    name,
		"display_name":    display,
		"namespace":       ns,
		"language":        orDefault(project.Language, "go"),
		"git_url":         project.GitURL,
		"git_ref_type":    gitRefType,
		"git_ref_value":   gitRefValue,
		"dockerfile_path": dockerfile,
		"cpu_request":     cpuReq,
		"cpu_limit":       cpuLim,
		"memory_request":  memReq,
		"memory_limit":    memLim,
		"replicas":        fmt.Sprintf("%d", replicas),
		"command":         res.Command,
		"args":            res.Args,
		"registry_type":   registry,
		"service_name":    name + "-service",
		"deployment_name": name + "-deployment",
		"app_label":       name,
		"ports_yaml":      formatPortsYAML(res.Ports),
	}
	portsJSON, _ := json.Marshal(res.Ports)
	vars["ports_json"] = string(portsJSON)

	for k, v := range parseProjectConfigMap(res.ProjectConfig) {
		vars[k] = v
	}
	return vars
}

func parseProjectConfigMap(raw string) map[string]string {
	out := map[string]string{}
	text := strings.TrimSpace(raw)
	if text == "" {
		return out
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		for k, v := range obj {
			out[k] = fmt.Sprint(v)
		}
		return out
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			out[line] = ""
			continue
		}
		out[strings.TrimSpace(line[:sep])] = strings.TrimSpace(line[sep+1:])
	}
	return out
}

func formatPortsYAML(ports []model.ProjectPort) string {
	if len(ports) == 0 {
		return `    - port: 80
      name: http-80
      protocol: TCP`
	}
	var b strings.Builder
	for i, p := range ports {
		container := p.ContainerPort
		if container == 0 {
			container = 80
		}
		service := p.ServicePort
		if service == 0 {
			service = container
		}
		protocol := orDefault(p.Protocol, "TCP")
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = fmt.Sprintf("http-%d", service)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "    - port: %d\n      targetPort: %d\n      name: %s\n      protocol: %s", service, container, name, protocol)
	}
	return b.String()
}

func escapeYAML(value string) string {
	if strings.ContainsAny(value, ":#{}[],&*?|>!%@`\"'\n") {
		b, _ := json.Marshal(value)
		return string(b)
	}
	return value
}

func defaultContent(kind string, project *model.Project, vars map[string]string) string {
	switch kind {
	case "Service":
		return defaultService(vars)
	case "Deployment":
		return defaultDeployment(project, vars)
	default:
		return defaultJenkinsfile(vars, project.Resources.EnableStaticScan)
	}
}

func defaultService(vars map[string]string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  annotations:
    sidecar.istio.io/inject: "false"
  labels:
    app: %s
spec:
  selector:
    app: %s
  ports:
%s
---
`, vars["service_name"], vars["namespace"], vars["service_name"], vars["app_label"], vars["ports_yaml"])
}

func defaultDeployment(project *model.Project, vars map[string]string) string {
	commandLines := nonEmptyLines(project.Resources.Command)
	argLines := nonEmptyLines(project.Resources.Args)
	configEntries := parseProjectConfigMap(project.Resources.ProjectConfig)

	var commandBlock, argsBlock, envBlock strings.Builder
	if len(commandLines) > 0 {
		commandBlock.WriteString("        command:\n")
		for _, c := range commandLines {
			fmt.Fprintf(&commandBlock, "          - %s\n", escapeYAML(c))
		}
	}
	if len(argLines) > 0 {
		argsBlock.WriteString("        args:\n")
		for _, a := range argLines {
			fmt.Fprintf(&argsBlock, "          - %s\n", escapeYAML(a))
		}
	}
	if len(configEntries) > 0 {
		envBlock.WriteString("        env:\n")
		keys := make([]string, 0, len(configEntries))
		for k := range configEntries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&envBlock, "          - name: %s\n            value: %s\n", escapeYAML(k), escapeYAML(configEntries[k]))
		}
	}

	ports := project.Resources.Ports
	var containerPorts strings.Builder
	if len(ports) == 0 {
		containerPorts.WriteString(`        - containerPort: 8080
          name: http
          protocol: TCP`)
	} else {
		for i, p := range ports {
			cp := p.ContainerPort
			if cp == 0 {
				cp = 8080
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				name = fmt.Sprintf("port-%d", cp)
			}
			if i > 0 {
				containerPorts.WriteByte('\n')
			}
			fmt.Fprintf(&containerPorts, "        - containerPort: %d\n          name: %s\n          protocol: %s", cp, name, orDefault(p.Protocol, "TCP"))
		}
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: %s
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: %s
          image: %s:latest
%s%s%s          ports:
%s
          resources:
            requests:
              cpu: %s
              memory: %s
            limits:
              cpu: %s
              memory: %s
---
`,
		vars["deployment_name"], vars["namespace"], vars["app_label"], vars["replicas"], vars["app_label"], vars["app_label"],
		vars["app_label"], vars["app_label"],
		commandBlock.String(), argsBlock.String(), envBlock.String(),
		containerPorts.String(),
		vars["cpu_request"], vars["memory_request"], vars["cpu_limit"], vars["memory_limit"],
	)
}

func defaultJenkinsfile(vars map[string]string, enableScan bool) string {
	scanStage := ""
	if enableScan {
		scanStage = fmt.Sprintf(`    stage('Sonar Scan') {
      steps {
        echo 'Static code scan for %s'
      }
    }
`, vars["name"])
	}
	return fmt.Sprintf(`pipeline {
  agent any
  environment {
    APP_NAME = '%s'
    GIT_URL = '%s'
    GIT_REF = '%s'
    DOCKERFILE = '%s'
    REGISTRY_TYPE = '%s'
    NAMESPACE = '%s'
  }
  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }
    stage('Build') {
      steps {
        echo "Building $%%{APP_NAME} from $%%{GIT_URL}#$%%{GIT_REF}"
        sh "docker build -f $%%{DOCKERFILE} -t $%%{APP_NAME}:latest ."
      }
    }
%s    stage('Push') {
      steps {
        echo "Push image to $%%{REGISTRY_TYPE}"
      }
    }
    stage('Deploy') {
      steps {
        echo "Deploy $%%{APP_NAME} to namespace $%%{NAMESPACE}"
      }
    }
  }
}
`, vars["name"], vars["git_url"], vars["git_ref_value"], vars["dockerfile_path"], vars["registry_type"], vars["namespace"], scanStage)
}

func filenameFor(kind, name string) string {
	switch kind {
	case "jenkinsFile":
		return "Jenkinsfile"
	case "Service":
		return name + "-service.yaml"
	default:
		return name + "-deployment.yaml"
	}
}

func nonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
