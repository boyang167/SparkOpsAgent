package service

import (
	"context"
	"fmt"
	"strings"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	workspacemodel "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"gopkg.in/yaml.v3"
)

type YAMLManifestRenderer struct{}

func NewManifestRenderer() *YAMLManifestRenderer {
	return &YAMLManifestRenderer{}
}

func (*YAMLManifestRenderer) Render(_ context.Context, project deployport.ProjectDeploymentContext, imageTag string) ([]runtimeport.Manifest, error) {
	repository := strings.TrimSuffix(strings.TrimSpace(project.ImageRepository), "/")
	imageTag = strings.TrimSpace(imageTag)
	if repository == "" || imageTag == "" {
		return nil, fmt.Errorf("deployment image repository and tag are required")
	}
	if strings.TrimSpace(project.ServiceYAML) == "" || strings.TrimSpace(project.DeploymentYAML) == "" {
		serviceYAML, deploymentYAML, err := generateWorkspaceDefaults(project, repository+":"+imageTag)
		if err != nil {
			return nil, err
		}
		project.ServiceYAML = serviceYAML
		project.DeploymentYAML = deploymentYAML
	}
	service, err := validateManifest(project.ServiceYAML, "Service")
	if err != nil {
		return nil, err
	}
	deployment, err := renderDeployment(project.DeploymentYAML, repository+":"+imageTag)
	if err != nil {
		return nil, err
	}
	return []runtimeport.Manifest{{YAML: service}, {YAML: deployment}}, nil
}

func generateWorkspaceDefaults(project deployport.ProjectDeploymentContext, image string) (string, string, error) {
	name, err := workspacemodel.ProjectNameFromDB(project.Name)
	if err != nil {
		return "", "", err
	}
	source := &workspacemodel.Project{
		ID: project.ProjectID, WorkspaceID: project.WorkspaceID, Name: name,
		Namespace: project.Namespace, TemplateID: "deployment-default",
		Resources: workspacemodel.ProjectResources{Replicas: 1},
	}
	artifacts := workspacequery.GenerateArtifacts(source, nil, workspacequery.TemplateRenderContext{
		ContainerImage: image,
	})
	var serviceYAML, deploymentYAML string
	for _, artifact := range artifacts {
		switch artifact.Kind {
		case "Service":
			serviceYAML = artifact.Content
		case "Deployment":
			deploymentYAML = artifact.Content
		}
	}
	if serviceYAML == "" || deploymentYAML == "" {
		return "", "", fmt.Errorf("workspace template generation did not produce Service and Deployment")
	}
	return serviceYAML, deploymentYAML, nil
}

func validateManifest(raw, expectedKind string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("generated %s manifest is empty", expectedKind)
	}
	var manifest map[string]any
	if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
		return "", fmt.Errorf("parse generated %s manifest: %w", expectedKind, err)
	}
	if stringValue(manifest["apiVersion"]) == "" || stringValue(manifest["kind"]) != expectedKind {
		return "", fmt.Errorf("generated %s manifest has invalid apiVersion or kind", expectedKind)
	}
	metadata, ok := manifest["metadata"].(map[string]any)
	if !ok || stringValue(metadata["name"]) == "" || stringValue(metadata["namespace"]) == "" {
		return "", fmt.Errorf("generated %s manifest has empty metadata", expectedKind)
	}
	return raw, nil
}

func renderDeployment(raw, image string) (string, error) {
	if _, err := validateManifest(raw, "Deployment"); err != nil {
		return "", err
	}
	var manifest map[string]any
	if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
		return "", err
	}
	spec, ok := manifest["spec"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("generated Deployment manifest has empty spec")
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("generated Deployment manifest has empty pod template")
	}
	podSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("generated Deployment manifest has empty pod spec")
	}
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) == 0 {
		return "", fmt.Errorf("generated Deployment manifest has no containers")
	}
	for _, item := range containers {
		container, ok := item.(map[string]any)
		if !ok || stringValue(container["name"]) == "" {
			return "", fmt.Errorf("generated Deployment manifest has invalid container")
		}
		container["image"] = image
	}
	for _, key := range []string{"initContainers", "containers"} {
		items, _ := podSpec[key].([]any)
		for _, item := range items {
			if container, ok := item.(map[string]any); ok {
				coerceContainerStrings(container)
			}
		}
	}
	rendered, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode generated Deployment manifest: %w", err)
	}
	return string(rendered), nil
}

// Kubernetes requires env values, command and args to be strings; unquoted YAML
// scalars such as `true` or `8080` decode as bool/int and are rejected on apply.
func coerceContainerStrings(container map[string]any) {
	for _, key := range []string{"command", "args"} {
		items, ok := container[key].([]any)
		if !ok {
			continue
		}
		for i, item := range items {
			items[i] = scalarString(item)
		}
	}
	envs, _ := container["env"].([]any)
	for _, item := range envs {
		env, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if value, exists := env["value"]; exists {
			env["value"] = scalarString(value)
		}
	}
}

func scalarString(value any) any {
	switch v := value.(type) {
	case nil:
		return ""
	case string, map[string]any, []any:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
