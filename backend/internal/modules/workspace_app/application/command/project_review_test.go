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

type reviewProjectRepo struct {
	project *model.Project
}

func (r *reviewProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *reviewProjectRepo) Update(_ context.Context, project *model.Project) error {
	r.project = project
	return nil
}
func (r *reviewProjectRepo) FindByID(context.Context, kernel.ProjectID) (*model.Project, error) {
	return r.project, nil
}
func (r *reviewProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *reviewProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) {
	return []*model.Project{r.project}, nil
}
func (r *reviewProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 1, nil
}

type reviewAccessChecker struct {
	allowed map[string]bool
}

func (c reviewAccessChecker) HasProjectAccess(
	_ context.Context,
	_ kernel.WorkspaceID,
	memberID kernel.MemberID,
	_ kernel.ProjectID,
	action string,
) (bool, error) {
	return c.allowed[memberID.String()+":"+action], nil
}

type reviewPermissionChecker struct {
	allowed map[string]bool
}

func (c reviewPermissionChecker) HasPermission(
	_ context.Context,
	_ kernel.WorkspaceID,
	memberID kernel.MemberID,
	permission string,
) (bool, error) {
	return c.allowed[memberID.String()+":"+permission], nil
}

func (c reviewPermissionChecker) RequirePermission(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	permission string,
) error {
	ok, err := c.HasPermission(ctx, wsID, memberID, permission)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrProjectReviewForbidden
	}
	return nil
}

func reviewReadyProject(t *testing.T, wsID kernel.WorkspaceID, creator kernel.MemberID) *model.Project {
	t.Helper()
	name, err := model.NewProjectName("review-demo")
	require.NoError(t, err)
	return model.NewProject(wsID, name, creator, model.CreateProjectInput{
		Name:           "review-demo",
		GitURL:         "https://git.example/review-demo.git",
		GitRefValue:    "main",
		DockerfilePath: "./Dockerfile",
		TemplateID:     "11111111-1111-1111-1111-111111111111",
	})
}

func TestSubmitProjectReviewRequiresProjectWriteAndSubmitPermission(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	actor := kernel.NewMemberID()
	project := reviewReadyProject(t, wsID, actor)
	repo := &reviewProjectRepo{project: project}
	access := reviewAccessChecker{allowed: map[string]bool{actor.String() + ":write": true}}
	permissions := reviewPermissionChecker{allowed: map[string]bool{}}
	handler := command.NewSubmitProjectReviewHandler(repo, access, permissions)

	_, err := handler.Handle(ctx, wsID, actor, project.ID)
	require.ErrorIs(t, err, domain.ErrProjectReviewForbidden)

	permissions.allowed[actor.String()+":project:submit"] = true
	updated, err := handler.Handle(ctx, wsID, actor, project.ID)
	require.NoError(t, err)
	require.Equal(t, model.ProjectReviewPending, updated.ReviewStatus)
}

func TestReviewProjectRequiresReadReviewPermissionAndDifferentActor(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	project := reviewReadyProject(t, wsID, submitter)
	require.NoError(t, project.SubmitReview(submitter))
	repo := &reviewProjectRepo{project: project}
	access := reviewAccessChecker{allowed: map[string]bool{reviewer.String() + ":read": true}}
	permissions := reviewPermissionChecker{allowed: map[string]bool{reviewer.String() + ":project:review": true}}
	handler := command.NewReviewProjectHandler(repo, access, permissions)

	updated, err := handler.Handle(ctx, wsID, reviewer, project.ID, model.ProjectReviewApproved, "")
	require.NoError(t, err)
	require.Equal(t, model.ProjectReviewApproved, updated.ReviewStatus)

	project = reviewReadyProject(t, wsID, submitter)
	require.NoError(t, project.SubmitReview(submitter))
	repo.project = project
	access.allowed[submitter.String()+":read"] = true
	permissions.allowed[submitter.String()+":project:review"] = true
	_, err = handler.Handle(ctx, wsID, submitter, project.ID, model.ProjectReviewApproved, "")
	require.ErrorIs(t, err, domain.ErrProjectReviewSelf)
}
