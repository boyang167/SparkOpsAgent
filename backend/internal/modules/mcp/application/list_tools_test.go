package application

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	governancedomain "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubEntitlementsProvider struct {
	ents entitlements.WorkspaceEntitlements
}

func (s stubEntitlementsProvider) Resolve(context.Context, kernel.WorkspaceID) (entitlements.WorkspaceEntitlements, error) {
	return s.ents, nil
}

type stubPermissionChecker struct {
	perms map[string]bool
}

func (s stubPermissionChecker) RequirePermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, perm string) error {
	if s.perms[perm] {
		return nil
	}
	return governancedomain.ErrPermissionDenied
}

func (s stubPermissionChecker) HasPermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, perm string) (bool, error) {
	return s.perms[perm], nil
}

func TestModulesAllowed(t *testing.T) {
	spec := domain.ToolSpec{Modules: []string{"knowledge"}}
	if !modulesAllowed(spec, entitlements.WorkspaceEntitlements{Knowledge: true}) {
		t.Fatal("expected knowledge module allowed")
	}
	if modulesAllowed(spec, entitlements.WorkspaceEntitlements{Knowledge: false}) {
		t.Fatal("expected knowledge module disabled")
	}
}

func TestModulesAllowedRequiresEnabledPlugin(t *testing.T) {
	spec := domain.ToolSpec{PluginID: "kubernetes"}
	if modulesAllowed(spec, entitlements.WorkspaceEntitlements{Plugins: []string{"kubernetes"}}) {
		// ok
	} else {
		t.Fatal("expected kubernetes plugin allowed")
	}
	if modulesAllowed(spec, entitlements.WorkspaceEntitlements{}) {
		t.Fatal("expected kubernetes plugin disabled")
	}
}

func mustWorkspaceID(t *testing.T, raw string) kernel.WorkspaceID {
	t.Helper()
	id, err := kernel.ParseWorkspaceID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustMemberID(t *testing.T, raw string) kernel.MemberID {
	t.Helper()
	id, err := kernel.ParseMemberID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestToolVisibleRequiresPermission(t *testing.T) {
	spec := domain.ToolSpec{
		Name:       "chat_with_agent",
		Permission: authz.PermChatUse,
		Modules:    []string{"agent"},
	}
	checker := stubPermissionChecker{perms: map[string]bool{authz.PermProjectRead: true}}
	ents := entitlements.WorkspaceEntitlements{Agent: true}

	visible, err := toolVisible(context.Background(), checker, ents, mustWorkspaceID(t, "c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9"), mustMemberID(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"), spec)
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Fatal("expected chat tool hidden without chat:use")
	}

	checker.perms[authz.PermChatUse] = true
	visible, err = toolVisible(context.Background(), checker, ents, mustWorkspaceID(t, "c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9"), mustMemberID(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"), spec)
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Fatal("expected chat tool visible with chat:use")
	}
}
