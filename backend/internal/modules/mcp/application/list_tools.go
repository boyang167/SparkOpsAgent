package application

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListToolsHandler struct {
	registry     *domain.Registry
	checker      authz.PermissionChecker
	entitlements WorkspaceEntitlementsProvider
}

func NewListToolsHandler(registry *domain.Registry, checker authz.PermissionChecker, entitlements WorkspaceEntitlementsProvider) *ListToolsHandler {
	return &ListToolsHandler{registry: registry, checker: checker, entitlements: entitlements}
}

func (h *ListToolsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
) ([]domain.ToolDefinition, error) {
	ents, err := h.entitlements.Resolve(ctx, wsID)
	if err != nil {
		return nil, err
	}

	out := make([]domain.ToolDefinition, 0, len(h.registry.All()))
	for _, spec := range h.registry.All() {
		visible, err := toolVisible(ctx, h.checker, ents, wsID, memberID, spec)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		out = append(out, spec.Definition())
	}
	return out, nil
}
