package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS agent").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&agentModel{}, &versionModel{})
}

type agentModel struct {
	ID               string `gorm:"column:id;primaryKey"`
	ProjectID        string
	Name             string
	Status           string
	CurrentVersionID *string
	postgres.AuditColumns
}

func (agentModel) TableName() string { return "agent.agents" }

type versionModel struct {
	ID               string `gorm:"column:id;primaryKey"`
	AgentID          string
	Version          int
	SystemPrompt     string
	ModelName        string
	ModelConfig      []byte `gorm:"type:jsonb;not null;default:'{}'"`
	ToolIDs          []byte `gorm:"type:jsonb;not null;default:'[]'"`
	KnowledgeBaseIDs []byte `gorm:"type:jsonb;not null;default:'[]'"`
	postgres.AuditColumns
}

func (versionModel) TableName() string { return "agent.agent_versions" }

type AgentRepository struct{ db *gorm.DB }

func NewAgentRepository(db *gorm.DB) repository.AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) Save(ctx context.Context, agent *model.Agent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current *string
		if agent.CurrentVersionID != nil {
			s := agent.CurrentVersionID.String()
			current = &s
		}
		row := agentModel{
			ID: agent.ID.String(), ProjectID: agent.ProjectID.String(),
			Name: agent.Name.String(), Status: string(agent.Status),
			CurrentVersionID: current,
			AuditColumns: postgres.AuditColumns{CreatedAt: agent.CreatedAt, UpdatedAt: time.Now().UTC()},
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}

		var existing []versionModel
		if err := tx.Where("agent_id = ?", agent.ID.String()).Find(&existing).Error; err != nil {
			return err
		}
		known := make(map[string]struct{}, len(existing))
		for _, v := range existing {
			known[v.ID] = struct{}{}
		}
		for _, ver := range agent.Versions() {
			if _, ok := known[ver.ID.String()]; ok {
				continue
			}
			cfg, err := json.Marshal(ver.ModelConfig)
			if err != nil {
				return err
			}
			tools, err := json.Marshal(ver.ToolIDs)
			if err != nil {
				return err
			}
			kbIDs, err := json.Marshal(ver.KnowledgeBaseIDs)
			if err != nil {
				return err
			}
			vrow := versionModel{
				ID: ver.ID.String(), AgentID: ver.AgentID.String(), Version: ver.Version,
				SystemPrompt: ver.SystemPrompt, ModelName: ver.ModelName,
				ModelConfig: cfg, ToolIDs: tools, KnowledgeBaseIDs: kbIDs,
				AuditColumns: postgres.AuditColumns{CreatedAt: ver.CreatedAt, UpdatedAt: ver.CreatedAt},
			}
			if err := tx.Create(&vrow).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AgentRepository) Update(ctx context.Context, agent *model.Agent) error {
	var current *string
	if agent.CurrentVersionID != nil {
		s := agent.CurrentVersionID.String()
		current = &s
	}
	return r.db.WithContext(ctx).Model(&agentModel{}).Where("id = ?", agent.ID.String()).Updates(map[string]any{
		"name":               agent.Name.String(),
		"status":             string(agent.Status),
		"current_version_id": current,
		"updated_at":         time.Now().UTC(),
	}).Error
}

func (r *AgentRepository) FindByID(ctx context.Context, id kernel.AgentID) (*model.Agent, error) {
	var row agentModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

func (r *AgentRepository) FindByProjectAndName(ctx context.Context, projectID kernel.ProjectID, name model.AgentName) (*model.Agent, error) {
	var row agentModel
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND name = ? AND status != ?", projectID.String(), name.String(), string(model.AgentStatusArchived)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(ctx, row)
}

func (r *AgentRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]*model.Agent, error) {
	var rows []agentModel
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND status != ?", projectID.String(), string(model.AgentStatusArchived)).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Agent, 0, len(rows))
	for _, row := range rows {
		agent, err := r.loadAggregate(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, agent)
	}
	return out, nil
}

func (r *AgentRepository) CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("agent.agents").
		Joins("JOIN workspace_app.projects p ON p.id = agent.agents.project_id").
		Where("p.workspace_id = ? AND agent.agents.status != ?", wsID.String(), string(model.AgentStatusArchived)).
		Count(&count).Error
	return int(count), err
}

func (r *AgentRepository) loadAggregate(ctx context.Context, row agentModel) (*model.Agent, error) {
	name, err := model.NewAgentName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseAgentID(row.ID)
	projectID, _ := kernel.ParseProjectID(row.ProjectID)
	agent := &model.Agent{
		ID: id, ProjectID: projectID, Name: name,
		Status: model.AgentStatus(row.Status), CreatedAt: row.CreatedAt,
	}
	if row.CurrentVersionID != nil {
		verID, _ := kernel.ParseAgentVersionID(*row.CurrentVersionID)
		agent.CurrentVersionID = &verID
	}

	var vrows []versionModel
	if err := r.db.WithContext(ctx).Where("agent_id = ?", row.ID).Order("version asc").Find(&vrows).Error; err != nil {
		return nil, err
	}
	versions := make([]*model.AgentVersion, 0, len(vrows))
	for _, vrow := range vrows {
		ver, err := rowToVersion(vrow)
		if err != nil {
			return nil, err
		}
		versions = append(versions, ver)
	}
	agent.SetVersions(versions)
	return agent, nil
}

func rowToVersion(row versionModel) (*model.AgentVersion, error) {
	var cfg model.ModelConfig
	if len(row.ModelConfig) > 0 {
		if err := json.Unmarshal(row.ModelConfig, &cfg); err != nil {
			return nil, err
		}
	}
	var tools []string
	if len(row.ToolIDs) > 0 {
		if err := json.Unmarshal(row.ToolIDs, &tools); err != nil {
			return nil, err
		}
	}
	var kbIDs []string
	if len(row.KnowledgeBaseIDs) > 0 {
		if err := json.Unmarshal(row.KnowledgeBaseIDs, &kbIDs); err != nil {
			return nil, err
		}
	}
	id, _ := kernel.ParseAgentVersionID(row.ID)
	agentID, _ := kernel.ParseAgentID(row.AgentID)
	return &model.AgentVersion{
		ID: id, AgentID: agentID, Version: row.Version,
		SystemPrompt: row.SystemPrompt, ModelName: row.ModelName,
		ModelConfig: cfg, ToolIDs: tools, KnowledgeBaseIDs: kbIDs, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.AgentRepository = (*AgentRepository)(nil)
