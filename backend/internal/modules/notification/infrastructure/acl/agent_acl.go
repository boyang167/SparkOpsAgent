package acl

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AgentACL struct{ db *gorm.DB }

func NewAgentACL(db *gorm.DB) *AgentACL { return &AgentACL{db: db} }

type agentInfo struct {
	Name      string
	ProjectID string
}

func (a *AgentACL) GetAgentInfo(ctx context.Context, agentID kernel.AgentID) (name, projectID string, err error) {
	var row agentInfo
	err = postgres.DB(ctx, a.db).Table("agent.agents").
		Select("name, project_id").
		Where("id = ?", agentID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", fmt.Errorf("agent not found")
	}
	if err != nil {
		return "", "", err
	}
	return row.Name, row.ProjectID, nil
}
