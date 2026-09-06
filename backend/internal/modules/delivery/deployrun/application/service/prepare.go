package service

import (
	"context"
	"fmt"
	"strings"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
)

type PrepareService struct {
	jobSuffix      string
	renderer       deployport.PipelineRenderer
	jenkinsFactory func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error)
}

type PrepareInput struct {
	GitRefValue     string
	ImageTag        string
	ImageRepository string
}

func NewPrepareService(jobSuffix string, renderer deployport.PipelineRenderer) *PrepareService {
	return &PrepareService{jobSuffix: jobSuffix, renderer: renderer}
}

// UseJenkinsFactory overrides Jenkins resolution; intended for tests.
func (s *PrepareService) UseJenkinsFactory(
	fn func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error),
) *PrepareService {
	s.jenkinsFactory = fn
	return s
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
		GitCredentialID:      StableCredentialID(project.WorkspaceID, project.Git.ID),
		RegistryCredentialID: StableCredentialID(project.WorkspaceID, project.Harbor.ID),
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
		Credentials:    BuildJenkinsCredentials(project.WorkspaceID, project),
	}
	executor, err := s.resolveJenkins(project.Jenkins)
	if err != nil {
		return fmt.Errorf("resolve project jenkins: %w", err)
	}
	if err := executor.CreateOrUpdate(ctx, spec); err != nil {
		return fmt.Errorf("upsert jenkins job: %w", err)
	}
	return nil
}

func (s *PrepareService) resolveJenkins(secret deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
	if s.jenkinsFactory != nil {
		return s.jenkinsFactory(secret)
	}
	return jenkinsExecutorFromDeploymentSecret(secret, s.jobSuffix)
}

func jenkinsExecutorFromDeploymentSecret(
	secret deployport.DeploymentSecret,
	jobSuffix string,
) (deliveryport.JenkinsExecutor, error) {
	return jenkinsinfra.NewExecutorFromConnection(jenkinsinfra.ConnectionConfig{
		URL:      secret.ServiceURL,
		User:     secret.Username,
		Password: secret.Credentials.Password,
		Token: jenkinsToken(
			secret.Credentials.Token,
			secret.Credentials.APIKey,
			secret.Credentials.Password,
		),
	}, jobSuffix)
}

func jenkinsToken(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
