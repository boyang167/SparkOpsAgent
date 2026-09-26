package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectVersionListItem struct {
	ID            string                `json:"id"`
	VersionTag    string                `json:"version_tag"`
	Status        string                `json:"status"`
	Images        []model.VersionImage  `json:"images"`
	WorkflowRunID string                `json:"workflow_run_id,omitempty"`
	ProductID     string                `json:"product_id,omitempty"`
	CreatedAt     string                `json:"created_at"`
}

type ProjectVersionDetail struct {
	ProjectVersionListItem
	GitRefType   string                  `json:"git_ref_type"`
	GitRefValue  string                  `json:"git_ref_value"`
	Manifests    []model.VersionManifest `json:"manifests"`
	ErrorMessage string                  `json:"error_message,omitempty"`
}

type ListProjectVersionsHandler struct {
	versions  repository.ProjectVersionRepository
	projects  repository.ProjectRepository
	wsChecker port.WorkspaceChecker
}

func NewListProjectVersionsHandler(
	versions repository.ProjectVersionRepository,
	projects repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
) *ListProjectVersionsHandler {
	return &ListProjectVersionsHandler{versions: versions, projects: projects, wsChecker: wsChecker}
}

func (h *ListProjectVersionsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	limit, offset int,
) ([]ProjectVersionListItem, error) {
	if err := h.ensureProjectAccess(ctx, wsID, memberID, projectID); err != nil {
		return nil, err
	}
	items, err := h.versions.ListByProject(ctx, projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectVersionListItem, 0, len(items))
	for _, item := range items {
		out = append(out, toVersionListItem(item))
	}
	return out, nil
}

type GetProjectVersionHandler struct {
	versions  repository.ProjectVersionRepository
	projects  repository.ProjectRepository
	wsChecker port.WorkspaceChecker
}

func NewGetProjectVersionHandler(
	versions repository.ProjectVersionRepository,
	projects repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
) *GetProjectVersionHandler {
	return &GetProjectVersionHandler{versions: versions, projects: projects, wsChecker: wsChecker}
}

func (h *GetProjectVersionHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	versionID kernel.ProjectVersionID,
) (*ProjectVersionDetail, error) {
	if err := h.ensureProjectAccess(ctx, wsID, memberID, projectID); err != nil {
		return nil, err
	}
	version, err := h.versions.FindByID(ctx, projectID, versionID)
	if err != nil {
		return nil, err
	}
	if version.WorkspaceID != wsID {
		return nil, domain.ErrProjectVersionNotFound
	}
	detail := &ProjectVersionDetail{
		ProjectVersionListItem: toVersionListItem(version),
		GitRefType:             version.GitRefType,
		GitRefValue:            version.GitRefValue,
		Manifests:              version.Manifests,
		ErrorMessage:           version.ErrorMessage,
	}
	return detail, nil
}

func (h *ListProjectVersionsHandler) ensureProjectAccess(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	project, err := h.projects.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if project.WorkspaceID != wsID {
		return domain.ErrProjectNotFound
	}
	return nil
}

func (h *GetProjectVersionHandler) ensureProjectAccess(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
) error {
	list := &ListProjectVersionsHandler{versions: h.versions, projects: h.projects, wsChecker: h.wsChecker}
	return list.ensureProjectAccess(ctx, wsID, memberID, projectID)
}

func toVersionListItem(version *model.ProjectVersion) ProjectVersionListItem {
	item := ProjectVersionListItem{
		ID: version.ID.String(), VersionTag: version.VersionTag,
		Status: string(version.Status), Images: version.Images,
		CreatedAt: version.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if version.WorkflowRunID != nil {
		item.WorkflowRunID = version.WorkflowRunID.String()
	}
	if version.ProductID != nil {
		item.ProductID = version.ProductID.String()
	}
	return item
}
