package application

import (
	"context"
	"fmt"
	"time"

	agentquery "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	runtimecommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	runtimerepo "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	knowledgequery "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuiltinDeps struct {
	ListProjects  *workspacequery.ListProjectsHandler
	GetProject    *workspacequery.GetProjectHandler
	ListKBs       *knowledgequery.ListKnowledgeBasesHandler
	SearchDocs    *knowledgequery.SearchDocumentsHandler
	ListAgents    *agentquery.ListAgentsHandler
	StartChat     *runtimecommand.StartChatHandler
	Runs          runtimerepo.RunRepository
	ChatWaitLimit time.Duration
}

func RegisterBuiltinTools(registry *domain.Registry, deps BuiltinDeps) {
	waitLimit := deps.ChatWaitLimit
	if waitLimit <= 0 {
		waitLimit = 30 * time.Second
	}

	registry.Register(domain.ToolSpec{
		Name:        "list_projects",
		Description: "List active projects in the current workspace.",
		InputSchema: domain.ObjectSchema(map[string]any{}),
		Permission:  authz.PermProjectRead,
		Tier:        domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, _ map[string]any) (any, error) {
		items, err := deps.ListProjects.Handle(ctx, wsID, memberID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "get_project",
		Description: "Get a project by ID in the current workspace.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"project_id": domain.StringProperty("Project UUID"),
		}, "project_id"),
		Permission: authz.PermProjectRead,
		Tier:       domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		projectID, err := parseStringArg(args, "project_id")
		if err != nil {
			return nil, err
		}
		pid, err := kernel.ParseProjectID(projectID)
		if err != nil {
			return nil, fmt.Errorf("invalid project_id")
		}
		item, err := deps.GetProject.Handle(ctx, wsID, memberID, pid)
		if err != nil {
			return nil, err
		}
		return map[string]any{"item": item}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "list_knowledge_bases",
		Description: "List knowledge bases available in the current workspace.",
		InputSchema: domain.ObjectSchema(map[string]any{}),
		Permission:  authz.PermProjectRead,
		Modules:     []string{"knowledge"},
		Tier:        domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, _ map[string]any) (any, error) {
		items, err := deps.ListKBs.Handle(ctx, wsID, memberID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "list_agents",
		Description: "List agents under a project.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"project_id": domain.StringProperty("Project UUID"),
		}, "project_id"),
		Permission: authz.PermAgentRead,
		Modules:    []string{"agent"},
		Tier:       domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
		projectID, err := parseStringArg(args, "project_id")
		if err != nil {
			return nil, err
		}
		pid, err := kernel.ParseProjectID(projectID)
		if err != nil {
			return nil, fmt.Errorf("invalid project_id")
		}
		items, err := deps.ListAgents.Handle(ctx, wsID, pid)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "search_knowledge",
		Description: "Search documents in a knowledge base.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"knowledge_base_id": domain.StringProperty("Knowledge base UUID"),
			"query":             domain.StringProperty("Search query"),
			"limit":             domain.IntegerProperty("Max results (default 5)"),
		}, "knowledge_base_id", "query"),
		Permission: authz.PermProjectRead,
		Modules:    []string{"knowledge"},
		Tier:       domain.ToolTierRead,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		kbID, err := parseStringArg(args, "knowledge_base_id")
		if err != nil {
			return nil, err
		}
		query, err := parseStringArg(args, "query")
		if err != nil {
			return nil, err
		}
		limit := parseIntArg(args, "limit", 5)
		id, err := kernel.ParseKnowledgeBaseID(kbID)
		if err != nil {
			return nil, fmt.Errorf("invalid knowledge_base_id")
		}
		items, err := deps.SearchDocs.Handle(ctx, wsID, memberID, id, query, limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "chat_with_agent",
		Description: "Send a message to a published agent. Returns immediately unless wait_seconds is set.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"agent_id":        domain.StringProperty("Agent UUID"),
			"message":         domain.StringProperty("User message"),
			"conversation_id": domain.StringProperty("Optional conversation UUID to continue"),
			"wait_seconds":    domain.IntegerProperty("Optional wait time up to 30 seconds for the reply"),
		}, "agent_id", "message"),
		Permission: authz.PermChatUse,
		Modules:    []string{"agent"},
		Tier:       domain.ToolTierAction,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		return chatWithAgent(ctx, deps, waitLimit, wsID, memberID, args)
	})

	registry.Register(domain.ToolSpec{
		Name:        "get_chat_run",
		Description: "Get the status and reply of an agent chat run.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"run_id": domain.StringProperty("Run UUID returned by chat_with_agent"),
		}, "run_id"),
		Permission: authz.PermChatUse,
		Modules:    []string{"agent"},
		Tier:       domain.ToolTierRead,
	}, func(ctx context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
		runIDRaw, err := parseStringArg(args, "run_id")
		if err != nil {
			return nil, err
		}
		runID, err := kernel.ParseRunID(runIDRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid run_id")
		}
		run, err := deps.Runs.FindByID(ctx, runID)
		if err != nil {
			return nil, err
		}
		return runPayload(run), nil
	})
}

func chatWithAgent(
	ctx context.Context,
	deps BuiltinDeps,
	maxWait time.Duration,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	args map[string]any,
) (any, error) {
	agentIDRaw, err := parseStringArg(args, "agent_id")
	if err != nil {
		return nil, err
	}
	message, err := parseStringArg(args, "message")
	if err != nil {
		return nil, err
	}
	agentID, err := kernel.ParseAgentID(agentIDRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id")
	}
	var conversationID *kernel.ConversationID
	if raw, ok := args["conversation_id"].(string); ok && raw != "" {
		id, err := kernel.ParseConversationID(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid conversation_id")
		}
		conversationID = &id
	}

	result, err := deps.StartChat.Handle(ctx, wsID, memberID, agentID, message, conversationID)
	if err != nil {
		return nil, err
	}

	waitSeconds := parseIntArg(args, "wait_seconds", 0)
	if waitSeconds > 30 {
		waitSeconds = 30
	}
	if waitSeconds <= 0 {
		return map[string]any{
			"run_id":          result.RunID.String(),
			"conversation_id": result.ConversationID.String(),
			"status":          "running",
		}, nil
	}

	waitLimit := time.Duration(waitSeconds) * time.Second
	if waitLimit > maxWait {
		waitLimit = maxWait
	}
	run, err := waitForRun(ctx, deps.Runs, result.RunID, waitLimit)
	if err != nil {
		return map[string]any{
			"run_id":          result.RunID.String(),
			"conversation_id": result.ConversationID.String(),
			"status":          "timeout",
			"message":         "Agent is still running; call get_chat_run to poll status.",
		}, nil
	}
	out := runPayload(run)
	out["conversation_id"] = result.ConversationID.String()
	return out, nil
}

func runPayload(run *model.Run) map[string]any {
	out := map[string]any{
		"run_id": run.ID.String(),
		"status": string(run.Status),
	}
	if run.Status == model.RunStatusCompleted {
		out["reply"] = run.Output
	}
	if run.Status == model.RunStatusFailed {
		out["error"] = "agent run failed"
	}
	return out
}

func waitForRun(ctx context.Context, runs runtimerepo.RunRepository, runID kernel.RunID, limit time.Duration) (*model.Run, error) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		run, err := runs.FindByID(ctx, runID)
		if err != nil {
			return nil, err
		}
		if run.Status == model.RunStatusCompleted || run.Status == model.RunStatusFailed {
			return run, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("run timeout")
}
