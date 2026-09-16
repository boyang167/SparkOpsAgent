package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectResourcesView struct {
	CPURequest            string              `json:"cpu_request,omitempty"`
	CPULimit              string              `json:"cpu_limit,omitempty"`
	MemoryRequest         string              `json:"memory_request,omitempty"`
	MemoryLimit           string              `json:"memory_limit,omitempty"`
	Replicas              int                 `json:"replicas,omitempty"`
	Ports                 []model.ProjectPort `json:"ports,omitempty"`
	Command               string              `json:"command,omitempty"`
	Args                  string              `json:"args,omitempty"`
	ProjectConfig         string              `json:"project_config,omitempty"`
	ServiceTemplateID     string              `json:"service_template_id,omitempty"`
	DeploymentTemplateID  string              `json:"deployment_template_id,omitempty"`
	JenkinsfileTemplateID string              `json:"jenkinsfile_template_id,omitempty"`
	RegistryType          string              `json:"registry_type,omitempty"`
	RegistrySecretID      string              `json:"registry_secret_id,omitempty"`
	SonarqubeSecretID     string              `json:"sonarqube_secret_id,omitempty"`
	EnableStaticScan      bool                `json:"enable_static_scan,omitempty"`
}

type ProjectView struct {
	ID                 string               `json:"id"`
	ProductID          string               `json:"product_id,omitempty"`
	Name               string               `json:"name"`
	DisplayName        string               `json:"display_name,omitempty"`
	Description        string               `json:"description"`
	Namespace          string               `json:"namespace,omitempty"`
	Language           string               `json:"language,omitempty"`
	GitURL             string               `json:"git_url,omitempty"`
	SecretID           string               `json:"secret_id,omitempty"`
	GitRefType         string               `json:"git_ref_type,omitempty"`
	GitRefValue        string               `json:"git_ref_value,omitempty"`
	DockerfilePath     string               `json:"dockerfile_path,omitempty"`
	TemplateID         string               `json:"template_id,omitempty"`
	DefaultClusterID   string               `json:"default_cluster_id,omitempty"`
	JenkinsSecretID    string               `json:"jenkins_secret_id,omitempty"`
	KubernetesSecretID string               `json:"kubernetes_secret_id,omitempty"`
	Resources          ProjectResourcesView `json:"resources,omitempty"`
	Status             string               `json:"status"`
	ReviewStatus       string               `json:"review_status"`
	SubmittedBy        string               `json:"submitted_by,omitempty"`
	SubmittedAt        string               `json:"submitted_at,omitempty"`
	ReviewedBy         string               `json:"reviewed_by,omitempty"`
	ReviewedAt         string               `json:"reviewed_at,omitempty"`
	ReviewComment      string               `json:"review_comment,omitempty"`
	CanSubmitReview    bool                 `json:"can_submit_review"`
	CanReview          bool                 `json:"can_review"`
	CreatedAt          string               `json:"created_at,omitempty"`
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
	isAdmin, err := hasPermission(ctx, h.governance, wsID, memberID, authz.PermGovernanceAdmin)
	if err != nil {
		return nil, err
	}
	canSeeAllProjects := isAdmin
	if !canSeeAllProjects {
		hasReview, reviewErr := hasPermission(ctx, h.governance, wsID, memberID, authz.PermProjectReview)
		if reviewErr != nil {
			return nil, reviewErr
		}
		canSeeAllProjects = hasReview
	}
	if !canSeeAllProjects {
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
		projects = filtered
	}
	return h.toViewsForActor(ctx, wsID, memberID, projects, isAdmin)
}

type GetProjectHandler struct {
	repo       repository.ProjectRepository
	memberRepo repository.ProjectMemberRepository
	wsChecker  port.WorkspaceChecker
	governance authz.PermissionChecker
}

func NewGetProjectHandler(
	repo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
) *GetProjectHandler {
	return &GetProjectHandler{repo: repo, memberRepo: memberRepo, wsChecker: wsChecker, governance: governance}
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
	isAdmin, err := hasPermission(ctx, h.governance, wsID, memberID, authz.PermGovernanceAdmin)
	if err != nil {
		return nil, err
	}
	if err := applyReviewCapabilities(ctx, &view, project, wsID, memberID, isAdmin, h.memberRepo, h.governance); err != nil {
		return nil, err
	}
	return &view, nil
}

func (h *ListProjectsHandler) toViewsForActor(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projects []*model.Project,
	isAdmin bool,
) ([]ProjectView, error) {
	views := ToViews(projects)
	for index, project := range projects {
		if err := applyReviewCapabilities(ctx, &views[index], project, wsID, memberID, isAdmin, h.memberRepo, h.governance); err != nil {
			return nil, err
		}
	}
	return views, nil
}

func applyReviewCapabilities(
	ctx context.Context,
	view *ProjectView,
	project *model.Project,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	isAdmin bool,
	memberRepo repository.ProjectMemberRepository,
	governance authz.PermissionChecker,
) error {
	hasProject, role, err := memberRepo.HasAccess(ctx, project.ID, memberID)
	if err != nil {
		return err
	}
	hasSubmit, err := hasPermission(ctx, governance, wsID, memberID, authz.PermProjectSubmit)
	if err != nil {
		return err
	}
	hasReview, err := hasPermission(ctx, governance, wsID, memberID, authz.PermProjectReview)
	if err != nil {
		return err
	}
	canWrite := isAdmin || (hasProject && (role == model.ProjectRoleLead || role == model.ProjectRoleMember))
	canRead := isAdmin || hasProject || hasReview
	view.CanSubmitReview = CanSubmitReview(project, canWrite, isAdmin || hasSubmit)
	view.CanReview = CanReviewProject(project, memberID, canRead, isAdmin || hasReview)
	return nil
}

func hasPermission(
	ctx context.Context,
	checker authz.PermissionChecker,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	permission string,
) (bool, error) {
	if checker == nil {
		return false, nil
	}
	return checker.HasPermission(ctx, wsID, memberID, permission)
}

func CanSubmitReview(project *model.Project, canWrite, hasSubmitPermission bool) bool {
	return project.Status == model.ProjectStatusActive &&
		project.IsConfigComplete() &&
		(project.ReviewStatus == model.ProjectReviewNone || project.ReviewStatus == model.ProjectReviewRejected) &&
		canWrite && hasSubmitPermission
}

func CanReviewProject(project *model.Project, actorID kernel.MemberID, canRead, hasReviewPermission bool) bool {
	return project.Status == model.ProjectStatusActive &&
		project.ReviewStatus == model.ProjectReviewPending &&
		(project.SubmittedBy == nil || *project.SubmittedBy != actorID) &&
		canRead && hasReviewPermission
}

func ToViews(projects []*model.Project) []ProjectView {
	out := make([]ProjectView, 0, len(projects))
	for _, p := range projects {
		out = append(out, ToView(p))
	}
	return out
}

func ToView(p *model.Project) ProjectView {
	view := ProjectView{
		ID:                 p.ID.String(),
		ProductID:          productIDString(p.ProductID),
		Name:               p.Name.String(),
		DisplayName:        p.DisplayName,
		Description:        p.Description,
		Namespace:          p.Namespace,
		Language:           p.Language,
		GitURL:             p.GitURL,
		SecretID:           p.SecretID,
		GitRefType:         p.GitRefType,
		GitRefValue:        p.GitRefValue,
		DockerfilePath:     p.DockerfilePath,
		TemplateID:         p.TemplateID,
		DefaultClusterID:   p.DefaultClusterID,
		JenkinsSecretID:    p.JenkinsSecretID,
		KubernetesSecretID: p.KubernetesSecretID,
		Resources: ProjectResourcesView{
			CPURequest:            p.Resources.CPURequest,
			CPULimit:              p.Resources.CPULimit,
			MemoryRequest:         p.Resources.MemoryRequest,
			MemoryLimit:           p.Resources.MemoryLimit,
			Replicas:              p.Resources.Replicas,
			Ports:                 p.Resources.Ports,
			Command:               p.Resources.Command,
			Args:                  p.Resources.Args,
			ProjectConfig:         p.Resources.ProjectConfig,
			ServiceTemplateID:     p.Resources.ServiceTemplateID,
			DeploymentTemplateID:  p.Resources.DeploymentTemplateID,
			JenkinsfileTemplateID: p.Resources.JenkinsfileTemplateID,
			RegistryType:          p.Resources.RegistryType,
			RegistrySecretID:      p.Resources.RegistrySecretID,
			SonarqubeSecretID:     p.Resources.SonarqubeSecretID,
			EnableStaticScan:      p.Resources.EnableStaticScan,
		},
		Status:        string(p.Status),
		ReviewStatus:  string(p.ReviewStatus),
		ReviewComment: p.ReviewComment,
		CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if p.SubmittedBy != nil {
		view.SubmittedBy = p.SubmittedBy.String()
	}
	if p.SubmittedAt != nil {
		view.SubmittedAt = p.SubmittedAt.Format(time.RFC3339)
	}
	if p.ReviewedBy != nil {
		view.ReviewedBy = p.ReviewedBy.String()
	}
	if p.ReviewedAt != nil {
		view.ReviewedAt = p.ReviewedAt.Format(time.RFC3339)
	}
	return view
}

func productIDString(id *kernel.ProductID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
