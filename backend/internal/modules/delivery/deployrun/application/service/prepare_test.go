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
	project := deployport.ProjectDeploymentContext{
		WorkspaceID: workspaceID,
		ProjectID:   kernel.NewProjectID(),
		Name:        "orders",
		Namespace:   " team-a ",
		Git: deployport.DeploymentSecret{
			ID: gitID, Username: "git-user",
			Credentials: deployport.DeploymentCredentials{Token: "git-token"},
		},
		Harbor: deployport.DeploymentSecret{
			ID: harborID, Username: "registry-user",
			Credentials: deployport.DeploymentCredentials{Password: "registry-password"},
		},
		Jenkins: deployport.DeploymentSecret{
			ID: jenkinsID, ServiceURL: "http://jenkins.example", Username: "jenkins-user",
			Credentials: deployport.DeploymentCredentials{APIKey: "jenkins-api-key"},
		},
		Sonar: &deployport.DeploymentSecret{
			ID:          sonarID,
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
		GitRefValue:          input.GitRefValue,
		ImageTag:             input.ImageTag,
		ImageRepository:      input.ImageRepository,
		GitCredentialID:      "sparkops-" + workspaceID.String() + "-" + gitID.String(),
		RegistryCredentialID: "sparkops-" + workspaceID.String() + "-" + harborID.String(),
	}, renderer.input)
	require.Equal(t, []deliveryport.JenkinsCredential{
		{ID: "sparkops-" + workspaceID.String() + "-" + gitID.String(), Username: "git-user", Secret: "git-token", Kind: "username-password"},
		{ID: "sparkops-" + workspaceID.String() + "-" + harborID.String(), Username: "registry-user", Secret: "registry-password", Kind: "username-password"},
		{ID: "sparkops-" + workspaceID.String() + "-" + jenkinsID.String(), Username: "jenkins-user", Secret: "jenkins-api-key", Kind: "username-password"},
		{ID: "sparkops-" + workspaceID.String() + "-" + sonarID.String(), Secret: "sonar-token", Kind: "secret-text"},
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
