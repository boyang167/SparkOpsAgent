package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentquery "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	runtimecommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	runtimerepo "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	knowledgequery "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ToolExecutor struct {
	listProjects  *workspacequery.ListProjectsHandler
	listKBs       *knowledgequery.ListKnowledgeBasesHandler
	searchDocs    *knowledgequery.SearchDocumentsHandler
	listAgents    *agentquery.ListAgentsHandler
	startChat     *runtimecommand.StartChatHandler
	runs          runtimerepo.RunRepository
	chatWaitLimit time.Duration
}

func NewToolExecutor(
	listProjects *workspacequery.ListProjectsHandler,
	listKBs *knowledgequery.ListKnowledgeBasesHandler,
	searchDocs *knowledgequery.SearchDocumentsHandler,
	listAgents *agentquery.ListAgentsHandler,
	startChat *runtimecommand.StartChatHandler,
	runs runtimerepo.RunRepository,
) *ToolExecutor {
	return &ToolExecutor{
		listProjects:  listProjects,
		listKBs:       listKBs,
		searchDocs:    searchDocs,
		listAgents:    listAgents,
		startChat:     startChat,
		runs:          runs,
		chatWaitLimit: 60 * time.Second,
	}
}

func (e *ToolExecutor) Call(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	name string,
	argsJSON json.RawMessage,
) (any, error) {
	var args map[string]any
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	switch name {
	case "list_projects":
		items, err := e.listProjects.Handle(ctx, wsID, memberID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	case "list_knowledge_bases":
		items, err := e.listKBs.Handle(ctx, wsID, memberID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	case "list_agents":
		projectID, err := parseStringArg(args, "project_id")
		if err != nil {
			return nil, err
		}
		pid, err := kernel.ParseProjectID(projectID)
		if err != nil {
			return nil, fmt.Errorf("invalid project_id")
		}
		items, err := e.listAgents.Handle(ctx, wsID, pid)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	case "search_knowledge":
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
		items, err := e.searchDocs.Handle(ctx, wsID, memberID, id, query, limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	case "chat_with_agent":
		return e.chatWithAgent(ctx, wsID, memberID, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (e *ToolExecutor) chatWithAgent(
	ctx context.Context,
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

	result, err := e.startChat.Handle(ctx, wsID, memberID, agentID, message, conversationID)
	if err != nil {
		return nil, err
	}

	run, err := e.waitForRun(ctx, result.RunID)
	if err != nil {
		return map[string]any{
			"run_id":          result.RunID.String(),
			"conversation_id": result.ConversationID.String(),
			"status":          "timeout",
			"message":         "Agent is still running; poll run status or open the conversation in SparkOps.",
		}, nil
	}

	out := map[string]any{
		"run_id":          result.RunID.String(),
		"conversation_id": result.ConversationID.String(),
		"status":          string(run.Status),
	}
	if run.Status == model.RunStatusCompleted {
		out["reply"] = run.Output
	}
	if run.Status == model.RunStatusFailed {
		out["error"] = "agent run failed"
	}
	return out, nil
}

func (e *ToolExecutor) waitForRun(ctx context.Context, runID kernel.RunID) (*model.Run, error) {
	deadline := time.Now().Add(e.chatWaitLimit)
	for time.Now().Before(deadline) {
		run, err := e.runs.FindByID(ctx, runID)
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

func parseStringArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key].(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	return raw, nil
}

func parseIntArg(args map[string]any, key string, defaultValue int) int {
	switch v := args[key].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return int(n)
		}
	}
	return defaultValue
}
