package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AgentView struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	CurrentVersionID string `json:"current_version_id,omitempty"`
}

type VersionView struct {
	ID               string            `json:"id"`
	Version          int               `json:"version"`
	SystemPrompt     string            `json:"system_prompt"`
	ModelName        string            `json:"model_name"`
	ModelConfig      model.ModelConfig `json:"model_config"`
	ToolIDs          []string          `json:"tool_ids"`
	KnowledgeBaseIDs []string          `json:"knowledge_base_ids"`
}

type AgentDetailView struct {
	AgentView
	Versions []VersionView `json:"versions"`
}

type ListAgentsHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
}

func NewListAgentsHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker) *ListAgentsHandler {
	return &ListAgentsHandler{repo: repo, projectChecker: projectChecker}
}

func (h *ListAgentsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]AgentView, error) {
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	agents, err := h.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]AgentView, 0, len(agents))
	for _, a := range agents {
		if a.Status == model.AgentStatusArchived {
			continue
		}
		out = append(out, toAgentView(a))
	}
	return out, nil
}

type GetAgentHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
}

func NewGetAgentHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker) *GetAgentHandler {
	return &GetAgentHandler{repo: repo, projectChecker: projectChecker}
}

func (h *GetAgentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, agentID kernel.AgentID) (*AgentDetailView, error) {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if agent.Status == model.AgentStatusArchived {
		return nil, domain.ErrAgentNotFound
	}
	view := &AgentDetailView{AgentView: toAgentView(agent)}
	for _, v := range agent.Versions() {
		view.Versions = append(view.Versions, VersionView{
			ID: v.ID.String(), Version: v.Version, SystemPrompt: v.SystemPrompt,
			ModelName: v.ModelName, ModelConfig: v.ModelConfig, ToolIDs: v.ToolIDs,
			KnowledgeBaseIDs: v.KnowledgeBaseIDs,
		})
	}
	return view, nil
}

func toAgentView(a *model.Agent) AgentView {
	view := AgentView{
		ID: a.ID.String(), ProjectID: a.ProjectID.String(),
		Name: a.Name.String(), Status: string(a.Status),
	}
	if a.CurrentVersionID != nil {
		view.CurrentVersionID = a.CurrentVersionID.String()
	}
	return view
}
