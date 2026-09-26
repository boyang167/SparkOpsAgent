package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceSprintProcessTemplate struct {
	ID          kernel.WorkspaceSprintProcessTemplateID
	WorkspaceID kernel.WorkspaceID
	Name        string
	TemplateKey string
	Description string
	Stages      []SprintStage
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BuiltinWorkspaceTemplateSpec struct {
	TemplateKey string
	Name        string
	Description string
}

func BuiltinWorkspaceTemplateSpecs() []BuiltinWorkspaceTemplateSpec {
	return []BuiltinWorkspaceTemplateSpec{
		{TemplateKey: "agile-4", Name: "标准敏捷（4 列）", Description: "开发 → 测试 → 预发 → 发布"},
		{TemplateKey: "lean-3", Name: "精简 Kanban（3 列）", Description: "待办 → 进行中 → 完成"},
		{TemplateKey: "devops-6", Name: "DevOps 全流程（6 列）", Description: "需求 → 开发 → CI → 测试 → 预发 → 生产"},
	}
}

func NewWorkspaceSprintProcessTemplate(
	workspaceID kernel.WorkspaceID,
	name, templateKey, description string,
	stages []SprintStage,
	createdBy kernel.MemberID,
) (*WorkspaceSprintProcessTemplate, error) {
	normalized, err := NormalizeSprintStages(stages)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &WorkspaceSprintProcessTemplate{
		ID: kernel.NewWorkspaceSprintProcessTemplateID(), WorkspaceID: workspaceID,
		Name: strings.TrimSpace(name), TemplateKey: strings.TrimSpace(templateKey),
		Description: strings.TrimSpace(description), Stages: normalized,
		CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}, nil
}
