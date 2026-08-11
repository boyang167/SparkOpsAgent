package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectResourcesView struct {
	CPURequest    string             `json:"cpu_request,omitempty"`
	CPULimit      string             `json:"cpu_limit,omitempty"`
	MemoryRequest string             `json:"memory_request,omitempty"`
	MemoryLimit   string             `json:"memory_limit,omitempty"`
	Replicas      int                `json:"replicas,omitempty"`
	Ports         []model.ProjectPort `json:"ports,omitempty"`
	Command       string             `json:"command,omitempty"`
	Args          string             `json:"args,omitempty"`
	ProjectConfig string             `json:"project_config,omitempty"`
}

type ProjectView struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	DisplayName    string               `json:"display_name,omitempty"`
	Description    string               `json:"description"`
	Namespace      string               `json:"namespace,omitempty"`
	Language       string               `json:"language,omitempty"`
	GitURL         string               `json:"git_url,omitempty"`
	SecretID       string               `json:"secret_id,omitempty"`
	GitRefType     string               `json:"git_ref_type,omitempty"`
	GitRefValue    string               `json:"git_ref_value,omitempty"`
	DockerfilePath string               `json:"dockerfile_path,omitempty"`
	TemplateID     string               `json:"template_id,omitempty"`
	Resources      ProjectResourcesView `json:"resources,omitempty"`
	Status         string               `json:"status"`
	CreatedAt      string               `json:"created_at,omitempty"`
}

type ListProjectsHandler struct {
	repo       repository.ProjectRepository
	memberRepo repository.ProjectMemberRepository
	wsChecker  port.WorkspaceChecker
	governance authz.PermissionChecker
}

func NewListProjectsHandler(
	repo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
) *ListProjectsHandler {
	return &ListProjectsHandler{repo: repo, memberRepo: memberRepo, wsChecker: wsChecker, governance: governance}
}

func (h *ListProjectsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]ProjectView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	projects, err := h.repo.ListByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if h.governance != nil {
		isAdmin, err := h.governance.HasPermission(ctx, wsID, memberID, authz.PermGovernanceAdmin)
		if err != nil {
			return nil, err
		}
		if isAdmin {
			return ToViews(projects), nil
		}
	}
	allowedIDs, err := h.memberRepo.ListProjectIDsForMember(ctx, wsID, memberID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id.String()] = struct{}{}
	}
	filtered := make([]*model.Project, 0, len(projects))
	for _, p := range projects {
		if _, ok := allowed[p.ID.String()]; ok {
			filtered = append(filtered, p)
		}
	}
	return ToViews(filtered), nil
}

type GetProjectHandler struct {
	repo      repository.ProjectRepository
	wsChecker port.WorkspaceChecker
}

func NewGetProjectHandler(repo repository.ProjectRepository, wsChecker port.WorkspaceChecker) *GetProjectHandler {
	return &GetProjectHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetProjectHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, projectID kernel.ProjectID) (*ProjectView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	if project.Status != model.ProjectStatusActive {
		return nil, domain.ErrProjectNotFound
	}
	view := ToView(project)
	return &view, nil
}

func ToViews(projects []*model.Project) []ProjectView {
	out := make([]ProjectView, 0, len(projects))
	for _, p := range projects {
		out = append(out, ToView(p))
	}
	return out
}

func ToView(p *model.Project) ProjectView {
	return ProjectView{
		ID:             p.ID.String(),
		Name:           p.Name.String(),
		DisplayName:    p.DisplayName,
		Description:    p.Description,
		Namespace:      p.Namespace,
		Language:       p.Language,
		GitURL:         p.GitURL,
		SecretID:       p.SecretID,
		GitRefType:     p.GitRefType,
		GitRefValue:    p.GitRefValue,
		DockerfilePath: p.DockerfilePath,
		TemplateID:     p.TemplateID,
		Resources: ProjectResourcesView{
			CPURequest:    p.Resources.CPURequest,
			CPULimit:      p.Resources.CPULimit,
			MemoryRequest: p.Resources.MemoryRequest,
			MemoryLimit:   p.Resources.MemoryLimit,
			Replicas:      p.Resources.Replicas,
			Ports:         p.Resources.Ports,
			Command:       p.Resources.Command,
			Args:          p.Resources.Args,
			ProjectConfig: p.Resources.ProjectConfig,
		},
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
