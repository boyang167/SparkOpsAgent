package command_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type updateWorkspaceChecker struct{}

func (updateWorkspaceChecker) MemberInWorkspace(context.Context, kernel.MemberID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}
func (updateWorkspaceChecker) CanCreateProject(context.Context, kernel.WorkspaceID, int) error {
	return nil
}
func (updateWorkspaceChecker) OrgIDForWorkspace(context.Context, kernel.WorkspaceID) (kernel.OrgID, error) {
	return kernel.NewOrgID(), nil
}

type clusterGrantChecker struct{ granted bool }

func (c clusterGrantChecker) WorkspaceClusterGranted(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (bool, error) {
	return c.granted, nil
}

func TestUpdateProjectRejectsUnauthorizedDefaultCluster(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	actor := kernel.NewMemberID()
	project := reviewReadyProject(t, wsID, actor)
	repo := &reviewProjectRepo{project: project}
	clusterID := kernel.NewClusterInfoID().String()
	handler := command.NewUpdateProjectHandler(repo, updateWorkspaceChecker{}, clusterGrantChecker{})

	_, err := handler.Handle(context.Background(), wsID, actor, project.ID, model.UpdateProjectInput{
		DefaultClusterID: &clusterID,
	})

	require.ErrorIs(t, err, domain.ErrClusterNotAuthorized)
	require.Empty(t, project.DefaultClusterID)
}
