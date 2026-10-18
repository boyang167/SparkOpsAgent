package domain

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ToolTier string

const (
	ToolTierDiscovery ToolTier = "discovery"
	ToolTierRead      ToolTier = "read"
	ToolTierAction    ToolTier = "action"
)

type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
	Permission  string
	Modules     []string
	PluginID    string
	Tier        ToolTier
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Tier        string         `json:"tier,omitempty"`
}

func (s ToolSpec) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        s.Name,
		Description: s.Description,
		InputSchema: s.InputSchema,
		Tier:        string(s.Tier),
	}
}

type ToolHandler func(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	args map[string]any,
) (any, error)

type toolEntry struct {
	spec    ToolSpec
	handler ToolHandler
}

type Registry struct {
	entries map[string]toolEntry
	order   []string
}

func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]toolEntry),
		order:   make([]string, 0, 8),
	}
}

func (r *Registry) Register(spec ToolSpec, handler ToolHandler) {
	if spec.Name == "" {
		panic("mcp tool name required")
	}
	if handler == nil {
		panic(fmt.Sprintf("mcp tool %q handler required", spec.Name))
	}
	if _, exists := r.entries[spec.Name]; exists {
		panic(fmt.Sprintf("mcp tool %q already registered", spec.Name))
	}
	r.entries[spec.Name] = toolEntry{spec: spec, handler: handler}
	r.order = append(r.order, spec.Name)
}

func (r *Registry) Lookup(name string) (ToolSpec, ToolHandler, bool) {
	entry, ok := r.entries[name]
	if !ok {
		return ToolSpec{}, nil, false
	}
	return entry.spec, entry.handler, true
}

func (r *Registry) All() []ToolSpec {
	out := make([]ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.entries[name].spec)
	}
	return out
}

// AllTools returns every registered tool definition (unfiltered).
func (r *Registry) AllTools() []ToolDefinition {
	specs := r.All()
	out := make([]ToolDefinition, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Definition())
	}
	return out
}
