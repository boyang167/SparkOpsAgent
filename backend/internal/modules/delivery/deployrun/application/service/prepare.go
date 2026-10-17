package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	harborinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/harbor"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
)

type HarborProjectEnsurer func(ctx context.Context, secret deployport.DeploymentSecret, projectName string) error

type PrepareService struct {
	jobSuffix      string
	renderer       deployport.PipelineRenderer
	jenkinsFactory func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error)
	harborEnsurer  HarborProjectEnsurer
}

type PrepareInput struct {
	GitRefType      string
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

// UseHarborEnsurer overrides Harbor project provisioning; intended for tests.
func (s *PrepareService) UseHarborEnsurer(fn HarborProjectEnsurer) *PrepareService {
	s.harborEnsurer = fn
	return s
}

func (s *PrepareService) Run(ctx context.Context, project deployport.ProjectDeploymentContext, input PrepareInput) error {
	if err := s.ensureHarborProject(ctx, project); err != nil {
		return err
	}
	if s.renderer == nil {
		return domain.ErrDeploymentPipelineEmpty
	}
	pipeline, err := s.renderer.Render(ctx, deployport.PipelineRenderRequest{
		WorkspaceID:          project.WorkspaceID,
		ProjectID:            project.ProjectID,
		GitRefType:           input.GitRefType,
		GitRefValue:          input.GitRefValue,
		ImageTag:             input.ImageTag,
		ImageRepository:      input.ImageRepository,
		GitCredentialID:      JenkinsCredentialID(project.Git, project.Namespace),
		RegistryCredentialID: JenkinsCredentialID(project.Harbor, project.Namespace),
		GitURLOverride:       ResolveCheckoutGitURL(project),
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

func (s *PrepareService) DisableJob(ctx context.Context, project deployport.ProjectDeploymentContext) error {
	executor, err := s.resolveJenkins(project.Jenkins)
	if err != nil {
		return fmt.Errorf("resolve project jenkins: %w", err)
	}
	disabler, ok := executor.(deliveryport.JenkinsJobDisabler)
	if !ok {
		return nil
	}
	return disabler.DisableJob(ctx, strings.TrimSpace(project.Namespace), strings.TrimSpace(project.Name))
}

func (s *PrepareService) PipelineDisabled(ctx context.Context, project deployport.ProjectDeploymentContext) (bool, error) {
	executor, err := s.resolveJenkins(project.Jenkins)
	if err != nil {
		return false, fmt.Errorf("resolve project jenkins: %w", err)
	}
	disabler, ok := executor.(deliveryport.JenkinsJobDisabler)
	if !ok {
		return true, nil
	}
	return disabler.IsJobDisabled(ctx, strings.TrimSpace(project.Namespace), strings.TrimSpace(project.Name))
}

func (s *PrepareService) ensureHarborProject(ctx context.Context, project deployport.ProjectDeploymentContext) error {
	if !strings.EqualFold(strings.TrimSpace(project.Harbor.ServiceType), "Harbor") {
		return nil
	}
	projectName := strings.TrimSpace(project.Namespace)
	if projectName == "" {
		return fmt.Errorf("ensure harbor project: project namespace is empty")
	}
	ensurer := s.harborEnsurer
	if ensurer == nil {
		ensurer = defaultHarborProjectEnsurer
	}
	if err := ensurer(ctx, project.Harbor, projectName); err != nil {
		return fmt.Errorf("ensure harbor project: %w", err)
	}
	return nil
}

func defaultHarborProjectEnsurer(
	ctx context.Context,
	secret deployport.DeploymentSecret,
	projectName string,
) error {
	if strings.TrimSpace(secret.ServiceURL) == "" {
		return fmt.Errorf("harbor service url is empty")
	}
	return harborinfra.EnsureProject(
		ctx,
		secret.ServiceURL,
		secret.Username,
		secret.Credentials.Password,
		projectName,
	)
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
