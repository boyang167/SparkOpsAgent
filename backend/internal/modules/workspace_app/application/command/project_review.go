package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SubmitProjectReviewHandler struct {
	repo        repository.ProjectRepository
	access      authz.ProjectAccessChecker
	permissions authz.PermissionChecker
}

func NewSubmitProjectReviewHandler(
	repo repository.ProjectRepository,
	access authz.ProjectAccessChecker,
	permissions authz.PermissionChecker,
) *SubmitProjectReviewHandler {
	return &SubmitProjectReviewHandler{repo: repo, access: access, permissions: permissions}
}

func (h *SubmitProjectReviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	actorID kernel.MemberID,
	projectID kernel.ProjectID,
) (*model.Project, error) {
	project, err := h.findProject(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAction(ctx, h.access, wsID, actorID, projectID, policy.ActionWrite); err != nil {
		return nil, err
	}
	if err := requireReviewPermission(ctx, h.permissions, wsID, actorID, authz.PermProjectSubmit); err != nil {
		return nil, err
	}
	if err := project.SubmitReview(actorID); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func (h *SubmitProjectReviewHandler) findProject(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (*model.Project, error) {
	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	return project, nil
}

type ReviewProjectHandler struct {
	repo        repository.ProjectRepository
	access      authz.ProjectAccessChecker
	permissions authz.PermissionChecker
}

func NewReviewProjectHandler(
	repo repository.ProjectRepository,
	access authz.ProjectAccessChecker,
	permissions authz.PermissionChecker,
) *ReviewProjectHandler {
	return &ReviewProjectHandler{repo: repo, access: access, permissions: permissions}
}

func (h *ReviewProjectHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	actorID kernel.MemberID,
	projectID kernel.ProjectID,
	decision model.ProjectReviewStatus,
	comment string,
) (*model.Project, error) {
	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	if err := requireProjectAction(ctx, h.access, wsID, actorID, projectID, policy.ActionRead); err != nil {
		return nil, err
	}
	if err := requireReviewPermission(ctx, h.permissions, wsID, actorID, authz.PermProjectReview); err != nil {
		return nil, err
	}
	if err := project.DecideReview(actorID, decision, comment); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func requireProjectAction(
	ctx context.Context,
	checker authz.ProjectAccessChecker,
	wsID kernel.WorkspaceID,
	actorID kernel.MemberID,
	projectID kernel.ProjectID,
	action string,
) error {
	allowed, err := checker.HasProjectAccess(ctx, wsID, actorID, projectID, action)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrProjectAccessDenied
	}
	return nil
}

func requireReviewPermission(
	ctx context.Context,
	checker authz.PermissionChecker,
	wsID kernel.WorkspaceID,
	actorID kernel.MemberID,
	permission string,
) error {
	allowed, err := checker.HasPermission(ctx, wsID, actorID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		allowed, err = checker.HasPermission(ctx, wsID, actorID, authz.PermGovernanceAdmin)
		if err != nil {
			return err
		}
	}
	if !allowed {
		return domain.ErrProjectReviewForbidden
	}
	return nil
}
