package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PrepareService struct {
	jenkins  deliveryport.JenkinsExecutor
	renderer deployport.PipelineRenderer
}

type PrepareInput struct {
	GitRefValue     string
	ImageTag        string
	ImageRepository string
}

func NewPrepareService(jenkins deliveryport.JenkinsExecutor, renderer deployport.PipelineRenderer) *PrepareService {
	return &PrepareService{jenkins: jenkins, renderer: renderer}
}

func (s *PrepareService) Run(ctx context.Context, project deployport.ProjectDeploymentContext, input PrepareInput) error {
	if s.renderer == nil {
		return domain.ErrDeploymentPipelineEmpty
	}
	pipeline, err := s.renderer.Render(ctx, deployport.PipelineRenderRequest{
		WorkspaceID:          project.WorkspaceID,
		ProjectID:            project.ProjectID,
		GitRefValue:          input.GitRefValue,
		ImageTag:             input.ImageTag,
		ImageRepository:      input.ImageRepository,
		GitCredentialID:      stableCredentialID(project.WorkspaceID, project.Git.ID),
		RegistryCredentialID: stableCredentialID(project.WorkspaceID, project.Harbor.ID),
	})
	if err != nil {
		return fmt.Errorf("render deployment pipeline: %w", err)
	}
	if strings.TrimSpace(pipeline) == "" {
		return domain.ErrDeploymentPipelineEmpty
	}
	spec := deliveryport.JenkinsJobSpec{
		Folder:         strings.TrimSpace(project.Namespace),
		Job:            strings.TrimSpace(project.Name),
		PipelineScript: pipeline,
		Credentials:    stableCredentials(project.WorkspaceID, project),
	}
	if err := s.jenkins.CreateOrUpdate(ctx, spec); err != nil {
		return fmt.Errorf("upsert jenkins job: %w", err)
	}
	return nil
}

func stableCredentialID(workspaceID kernel.WorkspaceID, secretID kernel.SecretID) string {
	if secretID.UUID == uuid.Nil {
		return ""
	}
	return "sparkops-" + workspaceID.String() + "-" + secretID.String()
}

func stableCredentials(workspaceID kernel.WorkspaceID, project deployport.ProjectDeploymentContext) []deliveryport.JenkinsCredential {
	secrets := []deployport.DeploymentSecret{project.Git, project.Harbor, project.Jenkins}
	if project.Sonar != nil {
		secrets = append(secrets, *project.Sonar)
	}
	credentials := make([]deliveryport.JenkinsCredential, 0, len(secrets))
	for _, secret := range secrets {
		if secret.ID.UUID == uuid.Nil {
			continue
		}
		value, kind := credentialValue(secret)
		if strings.TrimSpace(value) == "" {
			continue
		}
		credentials = append(credentials, deliveryport.JenkinsCredential{
			ID:       stableCredentialID(workspaceID, secret.ID),
			Username: secret.Username,
			Secret:   value,
			Kind:     kind,
		})
	}
	return credentials
}

func credentialValue(secret deployport.DeploymentSecret) (string, string) {
	credentials := secret.Credentials
	if strings.TrimSpace(credentials.PrivateKey) != "" {
		return credentials.PrivateKey, "ssh-private-key"
	}
	values := []string{credentials.Password, credentials.Token, credentials.APIKey}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			if strings.TrimSpace(secret.Username) != "" {
				return value, "username-password"
			}
			return value, "secret-text"
		}
	}
	return "", ""
}
