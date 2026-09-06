package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	deployservice "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubProjectLister struct {
	targets []deployacl.ProjectCredentialSyncTarget
	project deployport.ProjectDeploymentContext
}

func (s *stubProjectLister) ListProjectsUsingSecret(context.Context, kernel.WorkspaceID, string) ([]deployacl.ProjectCredentialSyncTarget, error) {
	return s.targets, nil
}

func (s *stubProjectLister) LoadJenkinsCredentialContext(context.Context, kernel.WorkspaceID, kernel.ProjectID) (deployport.ProjectDeploymentContext, error) {
	return s.project, nil
}

type syncRecordingJenkins struct {
	folder      string
	credentials []deliveryport.JenkinsCredential
	err         error
}

func (j *syncRecordingJenkins) CreateOrUpdate(context.Context, deliveryport.JenkinsJobSpec) error {
	return nil
}

func (j *syncRecordingJenkins) SyncCredentials(_ context.Context, folder string, credentials []deliveryport.JenkinsCredential) error {
	j.folder = folder
	j.credentials = credentials
	return j.err
}

func (j *syncRecordingJenkins) RunStreaming(context.Context, kernel.BuildID, deliveryport.BuildRequest, deliveryport.LogPublisher) (string, error) {
	return "", nil
}

func TestCredentialSyncServiceSyncsAffectedProjects(t *testing.T) {
	workspaceID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	gitID := kernel.NewSecretID()
	harborID := kernel.NewSecretID()
	jenkinsID := kernel.NewSecretID()
	recorder := &syncRecordingJenkins{}
	lister := &stubProjectLister{
		targets: []deployacl.ProjectCredentialSyncTarget{{ProjectID: projectID, Namespace: "team-a"}},
		project: deployport.ProjectDeploymentContext{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			Namespace:   "team-a",
			Git: deployport.DeploymentSecret{
				ID: gitID, Username: "git-user",
				Credentials: deployport.DeploymentCredentials{Token: "git-token"},
			},
			Harbor: deployport.DeploymentSecret{
				ID: harborID, Username: "registry-user",
				Credentials: deployport.DeploymentCredentials{Password: "registry-password"},
			},
			Jenkins: deployport.DeploymentSecret{
				ID: jenkinsID, Username: "jenkins-user",
				Credentials: deployport.DeploymentCredentials{APIKey: "jenkins-api-key"},
			},
		},
	}
	service := deployservice.NewCredentialSyncService(lister, jenkinsinfra.NewStaticProvider(recorder), 0, nil)

	err := service.SyncForSecret(context.Background(), workspaceID, gitID, "GitLab")
	require.NoError(t, err)
	require.Equal(t, "team-a", recorder.folder)
	require.Len(t, recorder.credentials, 3)
	require.Equal(t, deployservice.StableCredentialID(workspaceID, gitID), recorder.credentials[0].ID)
}

func TestCredentialSyncServiceSkipsNonJenkinsSecretTypes(t *testing.T) {
	recorder := &syncRecordingJenkins{}
	lister := &stubProjectLister{
		targets: []deployacl.ProjectCredentialSyncTarget{{ProjectID: kernel.NewProjectID(), Namespace: "team-a"}},
	}
	service := deployservice.NewCredentialSyncService(lister, jenkinsinfra.NewStaticProvider(recorder), 0, nil)

	err := service.SyncForSecret(context.Background(), kernel.NewWorkspaceID(), kernel.NewSecretID(), "Kubernetes")
	require.NoError(t, err)
	require.Empty(t, recorder.credentials)
}

func TestCredentialSyncServiceReturnsJoinedProjectErrors(t *testing.T) {
	workspaceID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	recorder := &syncRecordingJenkins{err: errors.New("jenkins unavailable")}
	lister := &stubProjectLister{
		targets: []deployacl.ProjectCredentialSyncTarget{{ProjectID: projectID, Namespace: "team-a"}},
		project: deployport.ProjectDeploymentContext{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			Namespace:   "team-a",
			Git: deployport.DeploymentSecret{
				ID: kernel.NewSecretID(), Username: "git-user",
				Credentials: deployport.DeploymentCredentials{Token: "git-token"},
			},
			Harbor: deployport.DeploymentSecret{
				ID: kernel.NewSecretID(), Username: "registry-user",
				Credentials: deployport.DeploymentCredentials{Password: "registry-password"},
			},
			Jenkins: deployport.DeploymentSecret{
				ID: kernel.NewSecretID(), Username: "jenkins-user",
				Credentials: deployport.DeploymentCredentials{APIKey: "jenkins-api-key"},
			},
		},
	}
	service := deployservice.NewCredentialSyncService(lister, jenkinsinfra.NewStaticProvider(recorder), 0, nil)

	err := service.SyncForSecret(context.Background(), workspaceID, kernel.NewSecretID(), "Harbor")
	require.ErrorContains(t, err, projectID.String())
	require.ErrorContains(t, err, "jenkins unavailable")
}
