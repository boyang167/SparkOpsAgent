package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildView struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	Status      string  `json:"status"`
	Branch      string  `json:"branch"`
	BuildNumber         int     `json:"build_number"`
	JenkinsJob          string  `json:"jenkins_job"`
	ConsoleLog          string  `json:"console_log,omitempty"`
	Error               string  `json:"error,omitempty"`
	RollbackOfBuildID   string  `json:"rollback_of_build_id,omitempty"`
	StartedAt           string  `json:"started_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type ListBuildsHandler struct {
	builds  repository.BuildRepository
	checker port.ProjectChecker
}

func NewListBuildsHandler(builds repository.BuildRepository, checker port.ProjectChecker) *ListBuildsHandler {
	return &ListBuildsHandler{builds: builds, checker: checker}
}

func (h *ListBuildsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]BuildView, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	builds, err := h.builds.ListByProject(ctx, projectID, 20)
	if err != nil {
		return nil, err
	}
	out := make([]BuildView, 0, len(builds))
	for _, build := range builds {
		out = append(out, toBuildView(build))
	}
	return out, nil
}

type GetBuildHandler struct {
	builds  repository.BuildRepository
	checker port.ProjectChecker
}

func NewGetBuildHandler(builds repository.BuildRepository, checker port.ProjectChecker) *GetBuildHandler {
	return &GetBuildHandler{builds: builds, checker: checker}
}

func (h *GetBuildHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, buildID kernel.BuildID) (*BuildView, error) {
	build, err := h.builds.FindByID(ctx, buildID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, build.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	view := toBuildView(build)
	return &view, nil
}

func toBuildView(build *model.Build) BuildView {
	view := BuildView{
		ID: build.ID.String(), ProjectID: build.ProjectID.String(),
		Status: string(build.Status), Branch: build.Branch,
		BuildNumber: build.BuildNumber, JenkinsJob: build.JenkinsJob,
		ConsoleLog: build.ConsoleLog, Error: build.Error,
		RollbackOfBuildID: build.RollbackOfBuildID,
		StartedAt: build.StartedAt.Format(time.RFC3339),
	}
	if build.CompletedAt != nil {
		s := build.CompletedAt.Format(time.RFC3339)
		view.CompletedAt = &s
	}
	return view
}
