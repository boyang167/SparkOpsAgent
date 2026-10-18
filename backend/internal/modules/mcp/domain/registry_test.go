package domain

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(ToolSpec{Name: "demo", Description: "demo tool", Tier: ToolTierRead}, func(
		_ context.Context,
		_ kernel.WorkspaceID,
		_ kernel.MemberID,
		_ map[string]any,
	) (any, error) {
		return map[string]any{"ok": true}, nil
	})

	spec, handler, ok := r.Lookup("demo")
	if !ok {
		t.Fatal("expected lookup ok")
	}
	if spec.Name != "demo" {
		t.Fatalf("unexpected spec name: %s", spec.Name)
	}
	if handler == nil {
		t.Fatal("expected handler")
	}
	if len(r.All()) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(r.All()))
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	noOp := func(context.Context, kernel.WorkspaceID, kernel.MemberID, map[string]any) (any, error) {
		return nil, nil
	}
	r.Register(ToolSpec{Name: "demo"}, noOp)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	r.Register(ToolSpec{Name: "demo"}, noOp)
}
