package query

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gorm.io/gorm"

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

type DeploymentPipelineRenderInput struct {
	GitRefValue          string
	ImageTag             string
	ImageRepository      string
	GitCredentialID      string
	RegistryCredentialID string
}

type GenerateProjectTemplatesHandler struct {
	projectRepo  repository.ProjectRepository
	templateRepo repository.TemplateRepository
	secretRepo   repository.SecretRepository
	wsChecker    port.WorkspaceChecker
	db           *gorm.DB
}

func NewGenerateProjectTemplatesHandler(
	projectRepo repository.ProjectRepository,
	templateRepo repository.TemplateRepository,
	secretRepo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	db *gorm.DB,
) *GenerateProjectTemplatesHandler {
	return &GenerateProjectTemplatesHandler{
		projectRepo:  projectRepo,
		templateRepo: templateRepo,
		secretRepo:   secretRepo,
		wsChecker:    wsChecker,
		db:           db,
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

	return GenerateArtifacts(project, byID, h.loadRenderContext(ctx, wsID, project)), nil
}

func (h *GenerateProjectTemplatesHandler) RenderDeploymentPipeline(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input DeploymentPipelineRenderInput,
) (string, error) {
	project, err := h.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.WorkspaceID != wsID || project.Status != model.ProjectStatusActive {
		return "", domain.ErrProjectNotFound
	}
	allTemplates, err := ListAccessibleTemplates(ctx, h.templateRepo, h.wsChecker, wsID)
	if err != nil {
		return "", err
	}
	byID := make(map[string]*model.Template, len(allTemplates))
	for _, item := range allTemplates {
		byID[item.ID.String()] = item
	}
	return RenderDeploymentPipelineArtifact(project, byID, h.loadRenderContext(ctx, wsID, project), input), nil
}

func RenderDeploymentPipelineArtifact(
	project *model.Project,
	byID map[string]*model.Template,
	renderCtx TemplateRenderContext,
	input DeploymentPipelineRenderInput,
) string {
	deploymentProject := *project
	deploymentProject.GitRefValue = strings.TrimSpace(input.GitRefValue)
	renderCtx.ContainerImage = strings.TrimSuffix(strings.TrimSpace(input.ImageRepository), "/") + ":" + strings.TrimSpace(input.ImageTag)
	renderCtx.GitCredentialID = strings.TrimSpace(input.GitCredentialID)
	renderCtx.RegistryCredentialID = strings.TrimSpace(input.RegistryCredentialID)
	for _, artifact := range GenerateArtifacts(&deploymentProject, byID, renderCtx) {
		if artifact.Kind == "jenkinsFile" {
			return artifact.Content
		}
	}
	return ""
}

type genPick struct {
	kind       string
	templateID string
}

func GenerateArtifacts(project *model.Project, byID map[string]*model.Template, renderCtx TemplateRenderContext) []GeneratedTemplateArtifact {
	vars := buildTemplateVars(project, renderCtx)
	picks := collectTemplatePicks(project, byID)

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
		usedCustomTemplate := source != "" && looksLikeManifest(source)
		if usedCustomTemplate {
			content = renderTemplateSource(source, project, pick.kind, renderCtx)
		} else {
			content = defaultContent(pick.kind, project, vars, renderCtx)
		}
		if usedCustomTemplate && pick.kind == "Deployment" {
			content = injectProjectConfigEnv(content, project.Resources.ProjectConfig)
		}
		if override, ok := project.Resources.ManifestOverrides[pick.kind]; ok && strings.TrimSpace(override) != "" {
			content = override
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

func collectTemplatePicks(project *model.Project, byID map[string]*model.Template) []genPick {
	resolved := map[string]string{}

	addPick := func(kind, id string) {
		id = strings.TrimSpace(id)
		if id == "" || resolved[kind] != "" {
			return
		}
		resolved[kind] = id
	}

	addByTemplateID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		kind := "Deployment"
		if t := byID[id]; t != nil {
			if k := normalizeKind(t.Kind); k != "" {
				kind = k
			}
		}
		addPick(kind, id)
	}

	addPick("Service", project.Resources.ServiceTemplateID)
	addPick("jenkinsFile", project.Resources.JenkinsfileTemplateID)
	addPick("Deployment", project.Resources.DeploymentTemplateID)
	addByTemplateID(project.TemplateID)

	picks := make([]genPick, 0, 3)
	if id, ok := resolved["Service"]; ok {
		picks = append(picks, genPick{kind: "Service", templateID: id})
	} else if resolved["Deployment"] != "" {
		picks = append(picks, genPick{kind: "Service", templateID: ""})
	}
	if id, ok := resolved["jenkinsFile"]; ok {
		picks = append(picks, genPick{kind: "jenkinsFile", templateID: id})
	}
	if id, ok := resolved["Deployment"]; ok {
		picks = append(picks, genPick{kind: "Deployment", templateID: id})
	}

	if len(picks) == 0 && strings.TrimSpace(project.TemplateID) != "" {
		kind := "Deployment"
		if t := byID[project.TemplateID]; t != nil {
			if k := normalizeKind(t.Kind); k != "" {
				kind = k
			}
		}
		picks = append(picks, genPick{kind: kind, templateID: project.TemplateID})
	}
	return picks
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
	mustacheRe       = regexp.MustCompile(`\{\{\s*\.?([a-zA-Z0-9_.-]+)\s*\}\}`)
	dollarRe         = regexp.MustCompile(`\$\{\s*([a-zA-Z0-9_.-]+)\s*\}`)
	percentRe        = regexp.MustCompile(`%([A-Z0-9_]+)%`)
	portRangeStartRe = regexp.MustCompile(`\{\{-?\s*range\s+\$k,\$v\s*:=\s*\.ports\s*\}\}`)
	resourceTypeIfRe = regexp.MustCompile(`\{\{-?\s*if\s+eq\s+\.resourceType\s+"service"\s*\}\}`)
)

func buildTemplatePorts(ports []model.ProjectPort, kind string) []map[string]interface{} {
	if len(ports) == 0 {
		port := 8080
		return []map[string]interface{}{
			{"port": port, "name": "http-80", "protocol": "TCP", "targetPort": port, "container_port": port, "service_port": port},
		}
	}
	out := make([]map[string]interface{}, 0, len(ports))
	for _, p := range ports {
		container := p.ContainerPort
		if container == 0 {
			container = 80
		}
		service := p.ServicePort
		if service == 0 {
			service = container
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = fmt.Sprintf("http-%d", service)
		}
		protocol := orDefault(p.Protocol, "TCP")
		portVal := service
		if kind == "Deployment" {
			portVal = container
		}
		out = append(out, map[string]interface{}{
			"port":           portVal,
			"name":           name,
			"protocol":       protocol,
			"targetPort":     container,
			"container_port": container,
			"service_port":   service,
		})
	}
	return out
}

func applyLegacyTemplateAliases(
	data map[string]interface{},
	vars map[string]string,
	project *model.Project,
	cpuReq, cpuLim, memReq, memLim string,
) {
	legacy := map[string]string{
		"memory":        memReq,
		"maxMemory":     memLim,
		"max_memory":    memLim,
		"cpu":           cpuReq,
		"max_cpu":       cpuLim,
		"git_addr":      project.GitURL,
		"git_buildpath": orDefault(strings.TrimSpace(project.DockerfilePath), "./Dockerfile"),
	}
	if project.Resources.EnableStaticScan {
		legacy["enable_static_scan"] = "true"
	} else {
		legacy["enable_static_scan"] = "false"
	}
	for k, v := range legacy {
		vars[k] = v
		data[k] = v
	}
	commands := nonEmptyLines(project.Resources.Command)
	args := nonEmptyLines(project.Resources.Args)
	data["command"] = commands
	data["commands"] = commands
	data["args"] = args
}

func buildTemplateData(project *model.Project, kind string, renderCtx TemplateRenderContext) map[string]interface{} {
	vars := buildTemplateVars(project, renderCtx)
	data := make(map[string]interface{}, len(vars)+12)
	for k, v := range vars {
		data[k] = v
	}
	replicas := project.Resources.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	data["replicas"] = replicas
	data["resourceType"] = resourceTypeForKind(kind)
	data["ports"] = buildTemplatePorts(project.Resources.Ports, kind)

	res := project.Resources
	cpuReq := orDefault(res.CPURequest, "500m")
	cpuLim := orDefault(res.CPULimit, "1000m")
	memReq := orDefault(res.MemoryRequest, "512Mi")
	memLim := orDefault(res.MemoryLimit, "1Gi")
	resourceMap := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    cpuReq,
			"memory": memReq,
		},
		"limits": map[string]interface{}{
			"cpu":    cpuLim,
			"memory": memLim,
		},
	}
	data["resources"] = resourceMap
	data["Resources"] = resourceMap
	projectEnv := buildProjectEnvData(project.Resources.ProjectConfig)
	data["project_env"] = projectEnv
	vars["project_env_yaml"] = formatProjectEnvYAML(buildProjectEnvEntries(project.Resources.ProjectConfig))
	applyRenderContextData(data, renderCtx, project.Name.String(), orDefault(project.GitRefValue, "latest"))
	applyLegacyTemplateAliases(data, vars, project, cpuReq, cpuLim, memReq, memLim)
	return data
}

func resourceTypeForKind(kind string) string {
	switch kind {
	case "Service":
		return "service"
	case "Deployment":
		return "deployment"
	case "jenkinsFile":
		return "jenkinsfile"
	default:
		return strings.ToLower(kind)
	}
}

func renderGoTemplate(source string, data map[string]interface{}) (string, error) {
	tpl, err := template.New("manifest").Parse(source)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func expandPortRangeBlocks(source string, ports []map[string]interface{}) string {
	loc := portRangeStartRe.FindStringIndex(source)
	if loc == nil {
		return source
	}
	prefix := source[:loc[0]]
	rest := source[loc[1]:]
	endIdx, endLen := portRangeBlockEnd(rest)
	block := rest[:endIdx]
	suffix := rest[endIdx+endLen:]

	var b strings.Builder
	b.WriteString(prefix)
	for _, p := range ports {
		line := block
		for key, val := range p {
			repl := fmt.Sprint(val)
			for _, token := range []string{
				fmt.Sprintf("{{$v.%s}}", key),
				fmt.Sprintf("{{ $v.%s }}", key),
				fmt.Sprintf("{{$v.%s}}", strings.ToLower(key)),
				fmt.Sprintf("{{ $v.%s }}", strings.ToLower(key)),
			} {
				line = strings.ReplaceAll(line, token, repl)
			}
		}
		b.WriteString(line)
	}
	b.WriteString(suffix)
	return b.String()
}

func portRangeBlockEnd(rest string) (idx int, skip int) {
	for _, marker := range []string{"{{- end }}", "{{- end}}", "{{ end }}", "{{end}}"} {
		if i := strings.Index(rest, marker); i >= 0 {
			return i, len(marker)
		}
	}
	for _, marker := range []string{"{{- if", "{{if", "{{- with", "{{with"} {
		if i := strings.Index(rest, marker); i >= 0 {
			return i, 0
		}
	}
	return len(rest), 0
}

func expandResourceTypeConditionals(source, kind string) string {
	if kind != "Service" {
		return resourceTypeIfRe.ReplaceAllString(source, "")
	}
	return resourceTypeIfRe.ReplaceAllString(source, "")
}

func renderTemplateSource(source string, project *model.Project, kind string, renderCtx TemplateRenderContext) string {
	data := buildTemplateData(project, kind, renderCtx)
	vars := buildTemplateVars(project, renderCtx)

	if strings.Contains(source, "{{") {
		if out, err := renderGoTemplate(source, data); err == nil {
			return renderPlaceholders(out, vars)
		}
		ports, _ := data["ports"].([]map[string]interface{})
		source = expandPortRangeBlocks(source, ports)
		source = expandResourceTypeConditionals(source, kind)
	}
	return renderPlaceholders(source, vars)
}

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

func buildTemplateVars(project *model.Project, renderCtx TemplateRenderContext) map[string]string {
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
	applyRenderContextVars(vars, renderCtx, name, gitRefValue)
	vars["memory"] = memReq
	vars["maxMemory"] = memLim
	vars["max_memory"] = memLim
	vars["cpu"] = cpuReq
	vars["max_cpu"] = cpuLim
	vars["git_addr"] = project.GitURL
	vars["git_buildpath"] = dockerfile
	if res.EnableStaticScan {
		vars["enable_static_scan"] = "true"
	} else {
		vars["enable_static_scan"] = "false"
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
		return `    - port: 8080
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

func defaultContent(kind string, project *model.Project, vars map[string]string, _ TemplateRenderContext) string {
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

	image := vars["image"]
	if image == "" {
		image = vars["app_label"] + ":latest"
	}
	pullSecret := vars["image_pull_secret_name"]
	pullSecretBlock := ""
	if pullSecret != "" {
		pullSecretBlock = fmt.Sprintf("      imagePullSecrets:\n        - name: %s\n", escapeYAML(pullSecret))
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
%s      containers:
        - name: %s
          image: %s
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
		pullSecretBlock,
		vars["app_label"], image,
		commandBlock.String(), argsBlock.String(), envBlock.String(),
		containerPorts.String(),
		vars["cpu_request"], vars["memory_request"], vars["cpu_limit"], vars["memory_limit"],
	)
}

func defaultJenkinsfile(vars map[string]string, enableScan bool) string {
	return renderBuiltinJenkinsfile(copyStringMap(vars), enableScan)
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
