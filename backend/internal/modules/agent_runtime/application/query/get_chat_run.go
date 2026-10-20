package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetChatRunHandler struct {
	conversations repository.ConversationRepository
	runs          repository.RunRepository
}

func NewGetChatRunHandler(
	conversations repository.ConversationRepository,
	runs repository.RunRepository,
) *GetChatRunHandler {
	return &GetChatRunHandler{conversations: conversations, runs: runs}
}

func (h *GetChatRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	runID kernel.RunID,
) (map[string]any, error) {
	run, err := h.runs.FindByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	conversation, err := h.conversations.FindByID(ctx, run.ConversationID)
	if err != nil {
		return nil, err
	}
	if conversation.WorkspaceID != wsID || conversation.MemberID != memberID {
		return nil, domain.ErrAccessDenied
	}
	return ChatRunView(run), nil
}

func ChatRunView(run *model.Run) map[string]any {
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
