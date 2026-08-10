package acl

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	agentdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AgentACL struct{ db *gorm.DB }

func NewAgentACL(db *gorm.DB) *AgentACL { return &AgentACL{db: db} }

type agentRow struct {
	ID               string
	ProjectID        string
	Status           string
	CurrentVersionID *string
}

type versionRow struct {
	SystemPrompt     string
	ModelName        string
	ModelConfig      []byte
	KnowledgeBaseIDs []byte
}

func (a *AgentACL) GetPublishedVersion(ctx context.Context, agentID kernel.AgentID) (*port.PublishedAgentVersion, error) {
	var row agentRow
	err := a.db.WithContext(ctx).Table("agent.agents").
		Select("id, project_id, status, current_version_id").
		Where("id = ?", agentID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, agentdomain.ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.Status != "published" || row.CurrentVersionID == nil {
		return nil, agentdomain.ErrAgentNotPublished
	}

	var ver versionRow
	err = a.db.WithContext(ctx).Table("agent.agent_versions").
		Select("system_prompt, model_name, model_config, knowledge_base_ids").
		Where("id = ?", *row.CurrentVersionID).
		First(&ver).Error
	if err != nil {
		return nil, agentdomain.ErrAgentNotPublished
	}

	var cfg struct {
		Temperature float64 `json:"temperature"`
		MaxTokens   int     `json:"max_tokens"`
	}
	if len(ver.ModelConfig) > 0 {
		_ = json.Unmarshal(ver.ModelConfig, &cfg)
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}

	var kbIDs []string
	if len(ver.KnowledgeBaseIDs) > 0 {
		_ = json.Unmarshal(ver.KnowledgeBaseIDs, &kbIDs)
	}

	projectID, _ := kernel.ParseProjectID(row.ProjectID)
	versionID, _ := kernel.ParseAgentVersionID(*row.CurrentVersionID)
	return &port.PublishedAgentVersion{
		AgentID: agentID, VersionID: versionID, ProjectID: projectID,
		SystemPrompt: ver.SystemPrompt, ModelName: ver.ModelName,
		Temperature: cfg.Temperature, MaxTokens: cfg.MaxTokens,
		KnowledgeBaseIDs: kbIDs,
	}, nil
}

func (a *AgentACL) AgentInWorkspace(ctx context.Context, agentID kernel.AgentID, wsID kernel.WorkspaceID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("agent.agents").
		Joins("JOIN workspace_app.projects p ON p.id = agent.agents.project_id").
		Where("agent.agents.id = ? AND p.workspace_id = ?", agentID.String(), wsID.String()).
		Count(&count).Error
	return count > 0, err
}

var _ port.AgentReader = (*AgentACL)(nil)
