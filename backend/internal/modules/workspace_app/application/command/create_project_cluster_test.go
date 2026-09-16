package command_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type createProjectRepo struct {
	saved bool
}

func (r *createProjectRepo) Save(context.Context, *model.Project) error {
	r.saved = true
	return nil
}
func (r *createProjectRepo) Update(context.Context, *model.Project) error { return nil }
func (r *createProjectRepo) FindByID(context.Context, kernel.ProjectID) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *createProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *createProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) {
	return nil, nil
}
func (r *createProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *createProjectRepo) ListByProduct(context.Context, kernel.ProductID) ([]*model.Project, error) {
	return nil, nil
}
func (r *createProjectRepo) CountByProduct(context.Context, kernel.ProductID) (int, error) {
	return 0, nil
}

type createProjectMemberRepo struct{}

func (createProjectMemberRepo) Save(context.Context, *model.ProjectMember) error { return nil }
func (createProjectMemberRepo) Find(context.Context, kernel.ProjectID, kernel.MemberID) (*model.ProjectMember, error) {
	return nil, nil
}
func (createProjectMemberRepo) ListByProject(context.Context, kernel.ProjectID) ([]repository.ProjectMemberRecord, error) {
	return nil, nil
}
func (createProjectMemberRepo) ListProjectIDsForMember(context.Context, kernel.WorkspaceID, kernel.MemberID) ([]kernel.ProjectID, error) {
	return nil, nil
}
func (createProjectMemberRepo) UpdateRole(context.Context, kernel.ProjectID, kernel.MemberID, string) error {
	return nil
}
func (createProjectMemberRepo) Remove(context.Context, kernel.ProjectID, kernel.MemberID) error {
	return nil
}
func (createProjectMemberRepo) HasAccess(context.Context, kernel.ProjectID, kernel.MemberID) (bool, string, error) {
	return false, "", nil
}

func TestCreateProjectRejectsUnauthorizedDefaultClusterBeforeSave(t *testing.T) {
	repo := &createProjectRepo{}
	handler := command.NewCreateProjectHandler(
		repo, createProjectMemberRepo{}, updateWorkspaceChecker{}, clusterGrantChecker{},
		nil, nil, eventbus.NewInMemory(),
	)

	_, err := handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(),
		model.CreateProjectInput{Name: "demo", DefaultClusterID: kernel.NewClusterInfoID().String()},
	)

	require.ErrorIs(t, err, domain.ErrClusterNotAuthorized)
	require.False(t, repo.saved)
}

func TestCreateProjectRejectsInvalidDefaultClusterIDBeforeSave(t *testing.T) {
	repo := &createProjectRepo{}
	handler := command.NewCreateProjectHandler(
		repo, createProjectMemberRepo{}, updateWorkspaceChecker{}, clusterGrantChecker{granted: true},
		nil, nil, eventbus.NewInMemory(),
	)

	_, err := handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(),
		model.CreateProjectInput{Name: "demo", DefaultClusterID: "not-a-uuid"},
	)

	require.ErrorIs(t, err, domain.ErrClusterNotAuthorized)
	require.False(t, repo.saved)
}
