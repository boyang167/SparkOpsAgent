package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ToolExecutor struct {
	registry     *domain.Registry
	checker      authz.PermissionChecker
	entitlements WorkspaceEntitlementsProvider
}

func NewToolExecutor(registry *domain.Registry, checker authz.PermissionChecker, entitlements WorkspaceEntitlementsProvider) *ToolExecutor {
	return &ToolExecutor{registry: registry, checker: checker, entitlements: entitlements}
}

func (e *ToolExecutor) Call(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	name string,
	argsJSON json.RawMessage,
) (any, error) {
	spec, handler, ok := e.registry.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	ents, err := e.entitlements.Resolve(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if err := requireToolAccess(ctx, e.checker, ents, wsID, memberID, spec); err != nil {
		return nil, err
	}

	var args map[string]any
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	return handler(ctx, wsID, memberID, args)
}
