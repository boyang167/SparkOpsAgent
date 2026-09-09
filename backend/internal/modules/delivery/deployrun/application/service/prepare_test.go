package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type recordingJenkins struct {
	spec  deliveryport.JenkinsJobSpec
	err   error
	calls int
}

func (j *recordingJenkins) CreateOrUpdate(_ context.Context, spec deliveryport.JenkinsJobSpec) error {
	j.calls++
	j.spec = spec
	return j.err
}

func (j *recordingJenkins) SyncCredentials(_ context.Context, folder string, credentials []deliveryport.JenkinsCredential) error {
	j.calls++
	j.spec.Folder = folder
	j.spec.Credentials = credentials
	return j.err
}

func (j *recordingJenkins) RunStreaming(context.Context, kernel.BuildID, deliveryport.BuildRequest, deliveryport.LogPublisher) (string, error) {
	return "", nil
}

type pipelineRenderer struct {
	content string
	err     error
	input   deployport.PipelineRenderRequest
}

func (r *pipelineRenderer) Render(_ context.Context, input deployport.PipelineRenderRequest) (string, error) {
	r.input = input
	return r.content, r.err
}

func TestPrepareUpsertsNamespaceJobPipelineAndStableCredentials(t *testing.T) {
	workspaceID := kernel.NewWorkspaceID()
	gitID := kernel.NewSecretID()
	harborID := kernel.NewSecretID()
	jenkinsID := kernel.NewSecretID()
	sonarID := kernel.NewSecretID()
	recorder := &recordingJenkins{}
	renderer := &pipelineRenderer{content: "pipeline { stages { stage('rendered') {} } }"}
	prepare := NewPrepareService("-build", renderer)
	prepare.jenkinsFactory = func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return recorder, nil
	}
	prepare.UseHarborEnsurer(func(context.Context, deployport.DeploymentSecret, string) error { return nil })
	project := deployport.ProjectDeploymentContext{
		WorkspaceID: workspaceID,
		ProjectID:   kernel.NewProjectID(),
		Name:        "orders",
		Namespace:   " team-a ",
		Git: deployport.DeploymentSecret{
			ID: gitID, Username: "git-user", ServiceType: "GitLab",
			Credentials: deployport.DeploymentCredentials{Token: "git-token"},
		},
		Harbor: deployport.DeploymentSecret{
			ID: harborID, Username: "registry-user", ServiceType: "Harbor",
			Credentials: deployport.DeploymentCredentials{Password: "registry-password"},
		},
		Jenkins: deployport.DeploymentSecret{
			ID: jenkinsID, ServiceURL: "http://jenkins.example", Username: "jenkins-user", ServiceType: "Jenkins",
			Credentials: deployport.DeploymentCredentials{APIKey: "jenkins-api-key"},
		},
		Sonar: &deployport.DeploymentSecret{
			ID:          sonarID,
			ServiceType: "SonarQube",
			Credentials: deployport.DeploymentCredentials{Token: "sonar-token"},
		},
	}
	input := PrepareInput{
		GitRefValue:     "release/2026-08",
		ImageTag:        "deploy-123",
		ImageRepository: "registry.example/team-a/orders",
	}

	require.NoError(t, prepare.Run(context.Background(), project, input))
	require.Equal(t, "team-a", recorder.spec.Folder)
	require.Equal(t, "orders", recorder.spec.Job)
	require.Equal(t, renderer.content, recorder.spec.PipelineScript)
	require.Equal(t, deployport.PipelineRenderRequest{
		WorkspaceID:          workspaceID,
		ProjectID:            project.ProjectID,
		GitRefType:           input.GitRefType,
		GitRefValue:          input.GitRefValue,
		ImageTag:             input.ImageTag,
		ImageRepository:      input.ImageRepository,
		GitCredentialID:        JenkinsCredentialID(project.Git, project.Namespace),
		RegistryCredentialID: JenkinsCredentialID(project.Harbor, project.Namespace),
	}, renderer.input)
	require.Equal(t, []deliveryport.JenkinsCredential{
		{
			ID:          JenkinsCredentialID(project.Git, project.Namespace),
			Description: JenkinsCredentialDescription(project.Git, project.Namespace),
			Username:    "git-user", Secret: "git-token", Kind: "username-password",
		},
		{
			ID:          JenkinsCredentialID(project.Harbor, project.Namespace),
			Description: JenkinsCredentialDescription(project.Harbor, project.Namespace),
			Username:    "registry-user", Secret: "registry-password", Kind: "username-password",
		},
		{
			ID:          JenkinsCredentialID(project.Jenkins, project.Namespace),
			Description: JenkinsCredentialDescription(project.Jenkins, project.Namespace),
			Username:    "jenkins-user", Secret: "jenkins-api-key", Kind: "username-password",
		},
		{
			ID:          JenkinsCredentialID(*project.Sonar, project.Namespace),
			Description: JenkinsCredentialDescription(*project.Sonar, project.Namespace),
			Secret: "sonar-token", Kind: "secret-text",
		},
	}, recorder.spec.Credentials)
}

func TestPrepareWrapsUpsertErrorWithoutCredentialValues(t *testing.T) {
	recorder := &recordingJenkins{err: errors.New("jenkins unavailable")}
	prepare := NewPrepareService("-build", &pipelineRenderer{content: "pipeline {}"})
	prepare.jenkinsFactory = func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return recorder, nil
	}
	project := deployport.ProjectDeploymentContext{
		WorkspaceID: kernel.NewWorkspaceID(),
		Name:        "orders",
		Namespace:   "team-a",
		Jenkinsfile: "pipeline {}",
		Jenkins: deployport.DeploymentSecret{
			ID: kernel.NewSecretID(), ServiceURL: "http://jenkins.example",
			Credentials: deployport.DeploymentCredentials{Token: "must-not-leak"},
		},
		Git: deployport.DeploymentSecret{
			ID:          kernel.NewSecretID(),
			Credentials: deployport.DeploymentCredentials{Token: "must-not-leak"},
		},
	}

	err := prepare.Run(context.Background(), project, PrepareInput{
		GitRefValue: "main", ImageTag: "tag", ImageRepository: "registry/orders",
	})

	require.ErrorContains(t, err, "upsert jenkins job")
	require.NotContains(t, err.Error(), "must-not-leak")
}

func TestPrepareRejectsEmptyRenderedPipelineWithoutUpsert(t *testing.T) {
	recorder := &recordingJenkins{}
	prepare := NewPrepareService("-build", &pipelineRenderer{content: " \n\t "})
	prepare.jenkinsFactory = func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return recorder, nil
	}
	prepare.UseHarborEnsurer(func(context.Context, deployport.DeploymentSecret, string) error { return nil })

	err := prepare.Run(context.Background(), deployport.ProjectDeploymentContext{
		WorkspaceID: kernel.NewWorkspaceID(),
		ProjectID:   kernel.NewProjectID(),
		Name:        "orders",
		Namespace:   "team-a",
		Jenkins: deployport.DeploymentSecret{
			ID: kernel.NewSecretID(), ServiceURL: "http://jenkins.example",
			Credentials: deployport.DeploymentCredentials{Token: "token"},
		},
	}, PrepareInput{GitRefValue: "main", ImageTag: "tag", ImageRepository: "registry/orders"})

	require.ErrorIs(t, err, deploydomain.ErrDeploymentPipelineEmpty)
	require.Zero(t, recorder.calls)
}

func TestPrepareEnsuresHarborProjectBeforeUpsert(t *testing.T) {
	recorder := &recordingJenkins{}
	renderer := &pipelineRenderer{content: "pipeline {}"}
	prepare := NewPrepareService("-build", renderer)
	prepare.jenkinsFactory = func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
		return recorder, nil
	}
	var ensured string
	prepare.UseHarborEnsurer(func(_ context.Context, secret deployport.DeploymentSecret, projectName string) error {
		ensured = projectName + "|" + secret.Username
		return nil
	})

	project := deployport.ProjectDeploymentContext{
		WorkspaceID: kernel.NewWorkspaceID(),
		ProjectID:   kernel.NewProjectID(),
		Name:        "orders",
		Namespace:   "team-a",
		Harbor: deployport.DeploymentSecret{
			ID: kernel.NewSecretID(), ServiceType: "Harbor", ServiceURL: "https://harbor.example",
			Username: "admin", Credentials: deployport.DeploymentCredentials{Password: "secret"},
		},
		Jenkins: deployport.DeploymentSecret{
			ID: kernel.NewSecretID(), ServiceURL: "http://jenkins.example",
			Credentials: deployport.DeploymentCredentials{Token: "token"},
		},
		Git: deployport.DeploymentSecret{
			ID: kernel.NewSecretID(), Credentials: deployport.DeploymentCredentials{Token: "git"},
		},
	}
	require.NoError(t, prepare.Run(context.Background(), project, PrepareInput{
		GitRefValue: "main", ImageTag: "tag", ImageRepository: "harbor.example/team-a/orders",
	}))
	require.Equal(t, "team-a|admin", ensured)
	require.Equal(t, 1, recorder.calls)
}
