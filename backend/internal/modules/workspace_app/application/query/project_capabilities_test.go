package query_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestReviewCapabilityState(t *testing.T) {
	submitter := kernel.NewMemberID()
	other := kernel.NewMemberID()
	project := &model.Project{
		Status:       model.ProjectStatusActive,
		ReviewStatus: model.ProjectReviewNone,
		GitURL:       "https://git.example/demo.git", GitRefValue: "main",
		DockerfilePath: "./Dockerfile", TemplateID: "template-id",
	}

	require.True(t, query.CanSubmitReview(project, true, true))
	require.False(t, query.CanReviewProject(project, other, true, true))

	project.ReviewStatus = model.ProjectReviewPending
	project.SubmittedBy = &submitter
	require.False(t, query.CanSubmitReview(project, true, true))
	require.True(t, query.CanReviewProject(project, other, true, true))
	require.False(t, query.CanReviewProject(project, submitter, true, true))

	project.ReviewStatus = model.ProjectReviewRejected
	require.True(t, query.CanSubmitReview(project, true, true))
	require.False(t, query.CanSubmitReview(project, false, true))
	require.False(t, query.CanSubmitReview(project, true, false))
}
