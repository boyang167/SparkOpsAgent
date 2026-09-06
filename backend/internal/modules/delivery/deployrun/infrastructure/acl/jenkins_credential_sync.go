package acl

import (
	"context"
	"encoding/json"
	"strings"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	workspace "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectCredentialSyncTarget struct {
	ProjectID kernel.ProjectID
	Namespace string
}

func (a *WorkspaceAppACL) ListProjectsUsingSecret(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	secretID string,
) ([]ProjectCredentialSyncTarget, error) {
	needle := strings.TrimSpace(secretID)
	if needle == "" {
		return nil, nil
	}
	var rows []struct {
		ID              string
		Namespace       string
		SecretID        *string
		JenkinsSecretID *string
		Resources       []byte
	}
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("id, namespace, secret_id, jenkins_secret_id, resources").
		Where("workspace_id = ? AND deleted_at IS NULL AND status = ?", wsID.String(), string(workspace.ProjectStatusActive)).
		Where("jenkins_secret_id IS NOT NULL AND jenkins_secret_id <> ''").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ProjectCredentialSyncTarget, 0)
	for _, row := range rows {
		if !projectReferencesSecret(row.SecretID, row.JenkinsSecretID, row.Resources, needle) {
			continue
		}
		projectID, err := kernel.ParseProjectID(row.ID)
		if err != nil {
			continue
		}
		out = append(out, ProjectCredentialSyncTarget{
			ProjectID: projectID,
			Namespace: strings.TrimSpace(row.Namespace),
		})
	}
	return out, nil
}

func projectReferencesSecret(gitSecretID, jenkinsSecretID *string, resources []byte, secretID string) bool {
	if value(gitSecretID) == secretID || value(jenkinsSecretID) == secretID {
		return true
	}
	if len(resources) == 0 {
		return false
	}
	var res workspace.ProjectResources
	if err := json.Unmarshal(resources, &res); err != nil {
		return false
	}
	return res.RegistrySecretID == secretID || res.SonarqubeSecretID == secretID
}

func (a *WorkspaceAppACL) LoadJenkinsCredentialContext(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (deployport.ProjectDeploymentContext, error) {
	project, err := a.loadProject(ctx, wsID, projectID)
	if err != nil {
		return deployport.ProjectDeploymentContext{}, err
	}
	var resources workspace.ProjectResources
	if len(project.Resources) > 0 {
		if err := json.Unmarshal(project.Resources, &resources); err != nil {
			return deployport.ProjectDeploymentContext{}, err
		}
	}
	git, err := a.loadSecret(ctx, wsID, value(project.SecretID), "GitLab", "GitHub")
	if err != nil {
		return deployport.ProjectDeploymentContext{}, err
	}
	harbor, err := a.loadSecret(ctx, wsID, resources.RegistrySecretID, "Harbor")
	if err != nil {
		return deployport.ProjectDeploymentContext{}, err
	}
	jenkins, err := a.loadSecret(ctx, wsID, value(project.JenkinsSecretID), "Jenkins")
	if err != nil {
		return deployport.ProjectDeploymentContext{}, err
	}
	var sonar *deployport.DeploymentSecret
	if resources.EnableStaticScan {
		loaded, err := a.loadSecret(ctx, wsID, resources.SonarqubeSecretID, "SonarQube")
		if err != nil {
			return deployport.ProjectDeploymentContext{}, err
		}
		sonar = &loaded
	}
	return deployport.ProjectDeploymentContext{
		WorkspaceID:      wsID,
		ProjectID:        projectID,
		Name:             project.Name,
		Namespace:        project.Namespace,
		EnableStaticScan: resources.EnableStaticScan,
		Git:              git,
		Harbor:           harbor,
		Jenkins:          jenkins,
		Sonar:            sonar,
	}, nil
}
