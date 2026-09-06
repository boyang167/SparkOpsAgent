package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateGeneratedTemplateHandler struct {
	repo      repository.ProjectRepository
	wsChecker port.WorkspaceChecker
	generate  *query.GenerateProjectTemplatesHandler
}

func NewUpdateGeneratedTemplateHandler(
	repo repository.ProjectRepository,
	wsChecker port.WorkspaceChecker,
	generate *query.GenerateProjectTemplatesHandler,
) *UpdateGeneratedTemplateHandler {
	return &UpdateGeneratedTemplateHandler{repo: repo, wsChecker: wsChecker, generate: generate}
}

func (h *UpdateGeneratedTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	kind string,
	content string,
) (query.GeneratedTemplateArtifact, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return query.GeneratedTemplateArtifact{}, err
	}
	if !ok {
		return query.GeneratedTemplateArtifact{}, domain.ErrWorkspaceAccessDenied
	}

	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return query.GeneratedTemplateArtifact{}, err
	}
	if project.WorkspaceID != wsID {
		return query.GeneratedTemplateArtifact{}, domain.ErrProjectNotFound
	}
	if err := project.SetManifestOverride(kind, content); err != nil {
		return query.GeneratedTemplateArtifact{}, err
	}
	if err := h.repo.Update(ctx, project); err != nil {
		return query.GeneratedTemplateArtifact{}, err
	}

	items, err := h.generate.Handle(ctx, wsID, memberID, projectID)
	if err != nil {
		return query.GeneratedTemplateArtifact{}, err
	}
	normalized := model.NormalizeManifestKind(kind)
	for _, item := range items {
		if item.Kind == normalized {
			return item, nil
		}
	}
	return query.GeneratedTemplateArtifact{}, domain.ErrInvalidManifestKind
}
