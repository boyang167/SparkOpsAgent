package model_test

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func mustName(t *testing.T, raw string) model.ProjectName {
	t.Helper()
	name, err := model.NewProjectName(raw)
	require.NoError(t, err)
	return name
}

func sampleInput(desc string) model.CreateProjectInput {
	return model.CreateProjectInput{Description: desc}
}

func TestProject_Archive(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput("desc"))
	require.NoError(t, p.Archive())
	require.Equal(t, model.ProjectStatusArchived, p.Status)
	require.ErrorIs(t, p.Archive(), domain.ErrProjectAlreadyArchived)
}

func TestProject_Rename_NotActive(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput(""))
	require.NoError(t, p.Archive())
	err := p.Rename(mustName(t, "new"))
	require.ErrorIs(t, err, domain.ErrProjectNotActive)
}

func TestProject_UpdateDetails(t *testing.T) {
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), kernel.NewMemberID(), sampleInput("old"))
	require.NoError(t, p.UpdateDetails(model.CreateProjectInput{Name: "new-name", Description: "new desc"}))
	require.Equal(t, "new-name", p.Name.String())
	require.Equal(t, "new desc", p.Description)
}

func TestNewProjectName_Invalid(t *testing.T) {
	_, err := model.NewProjectName("   ")
	require.ErrorIs(t, err, domain.ErrInvalidProjectName)
	_, err = model.NewProjectName("Bad_Name")
	require.ErrorIs(t, err, domain.ErrInvalidProjectName)
}

func completeInput() model.CreateProjectInput {
	return model.CreateProjectInput{
		Name:           "demo",
		GitURL:         "https://git.example/demo.git",
		GitRefValue:    "main",
		DockerfilePath: "./Dockerfile",
		TemplateID:     "11111111-1111-1111-1111-111111111111",
	}
}

func TestProject_SubmitReviewRequiresCompleteConfiguration(t *testing.T) {
	actor := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), actor, sampleInput(""))

	require.ErrorIs(t, p.SubmitReview(actor), domain.ErrProjectConfigIncomplete)
}

func TestProject_SubmitAndApproveReview(t *testing.T) {
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	input := completeInput()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, input)

	require.NoError(t, p.SubmitReview(submitter))
	require.Equal(t, model.ProjectReviewPending, p.ReviewStatus)
	require.Equal(t, submitter, *p.SubmittedBy)
	require.NotNil(t, p.SubmittedAt)

	require.NoError(t, p.DecideReview(reviewer, model.ProjectReviewApproved, "looks good"))
	require.Equal(t, model.ProjectReviewApproved, p.ReviewStatus)
	require.Equal(t, reviewer, *p.ReviewedBy)
	require.Equal(t, "looks good", p.ReviewComment)
}

func TestProject_RejectReviewRequiresCommentAndAllowsResubmit(t *testing.T) {
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))

	require.ErrorIs(
		t,
		p.DecideReview(reviewer, model.ProjectReviewRejected, "   "),
		domain.ErrProjectReviewCommentRequired,
	)
	require.NoError(t, p.DecideReview(reviewer, model.ProjectReviewRejected, "缺少扫描配置"))
	require.Equal(t, model.ProjectReviewRejected, p.ReviewStatus)
	require.NoError(t, p.SubmitReview(submitter))
	require.Equal(t, model.ProjectReviewPending, p.ReviewStatus)
	require.Nil(t, p.ReviewedBy)
	require.Nil(t, p.ReviewedAt)
	require.Empty(t, p.ReviewComment)
}

func TestProject_ReviewerCannotReviewOwnSubmission(t *testing.T) {
	submitter := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))

	require.ErrorIs(
		t,
		p.DecideReview(submitter, model.ProjectReviewApproved, ""),
		domain.ErrProjectReviewSelf,
	)
}

func TestProject_PendingReviewLocksConfigurationButAllowsDescription(t *testing.T) {
	submitter := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))

	changed := completeInput()
	changed.GitRefValue = "release"
	require.ErrorIs(t, p.UpdateDetails(changed), domain.ErrProjectReviewLocked)

	metadataOnly := completeInput()
	metadataOnly.DisplayName = "新显示名"
	metadataOnly.Description = "新描述"
	require.NoError(t, p.UpdateDetails(metadataOnly))
	require.Equal(t, model.ProjectReviewPending, p.ReviewStatus)
}

func TestProject_PartialMetadataPatchPreservesConfigurationAndReview(t *testing.T) {
	submitter := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))
	description := "只更新描述"

	require.NoError(t, p.ApplyPatch(model.UpdateProjectInput{Description: &description}))
	require.Equal(t, description, p.Description)
	require.Equal(t, "https://git.example/demo.git", p.GitURL)
	require.Equal(t, model.ProjectReviewPending, p.ReviewStatus)
}

func TestProject_PartialConfigurationPatchResetsApprovedReview(t *testing.T) {
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))
	require.NoError(t, p.DecideReview(reviewer, model.ProjectReviewApproved, "ok"))
	gitRef := "release"

	require.NoError(t, p.ApplyPatch(model.UpdateProjectInput{GitRefValue: &gitRef}))
	require.Equal(t, gitRef, p.GitRefValue)
	require.Equal(t, "https://git.example/demo.git", p.GitURL)
	require.Equal(t, model.ProjectReviewNone, p.ReviewStatus)
}

func TestProject_ApprovedConfigurationChangeResetsReview(t *testing.T) {
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))
	require.NoError(t, p.DecideReview(reviewer, model.ProjectReviewApproved, "ok"))

	changed := completeInput()
	changed.GitRefValue = "release"
	require.NoError(t, p.UpdateDetails(changed))
	require.Equal(t, model.ProjectReviewNone, p.ReviewStatus)
	require.Nil(t, p.ReviewedBy)
	require.Nil(t, p.ReviewedAt)
	require.Empty(t, p.ReviewComment)
}

func TestProject_SubmitReviewRejectsInvalidStates(t *testing.T) {
	submitter := kernel.NewMemberID()
	reviewer := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))
	require.ErrorIs(t, p.SubmitReview(submitter), domain.ErrProjectReviewInvalidState)
	require.NoError(t, p.DecideReview(reviewer, model.ProjectReviewApproved, ""))
	require.ErrorIs(t, p.SubmitReview(submitter), domain.ErrProjectReviewInvalidState)
	require.ErrorIs(
		t,
		p.DecideReview(reviewer, model.ProjectReviewApproved, ""),
		domain.ErrProjectReviewInvalidState,
	)

	require.NoError(t, p.Archive())
	require.ErrorIs(t, p.DecideReview(reviewer, model.ProjectReviewApproved, ""), domain.ErrProjectNotActive)
}

func TestProjectReviewTimestampsAreUTC(t *testing.T) {
	submitter := kernel.NewMemberID()
	p := model.NewProject(kernel.NewWorkspaceID(), mustName(t, "demo"), submitter, completeInput())
	require.NoError(t, p.SubmitReview(submitter))
	require.Equal(t, time.UTC, p.SubmittedAt.Location())
}
