package model

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	MinSprintStages = 2
	MaxSprintStages = 12
)

var sprintStageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// NormalizeSprintStages validates and normalizes stage definitions for process/sprint updates.
func NormalizeSprintStages(stages []SprintStage) ([]SprintStage, error) {
	if len(stages) < MinSprintStages || len(stages) > MaxSprintStages {
		return nil, domain.ErrInvalidSprintProcess
	}
	keys := map[string]struct{}{}
	ids := map[string]struct{}{}
	out := make([]SprintStage, 0, len(stages))
	for i, stage := range stages {
		key := strings.ToLower(strings.TrimSpace(stage.Key))
		name := strings.TrimSpace(stage.Name)
		if name == "" || key == "" || !sprintStageKeyPattern.MatchString(key) {
			return nil, domain.ErrInvalidSprintProcess
		}
		if _, ok := keys[key]; ok {
			return nil, domain.ErrInvalidSprintProcess
		}
		keys[key] = struct{}{}

		id := strings.TrimSpace(stage.ID)
		if id == "" {
			id = uuid.NewString()
		}
		if _, err := uuid.Parse(id); err != nil {
			return nil, domain.ErrInvalidSprintProcess
		}
		if _, ok := ids[id]; ok {
			return nil, domain.ErrInvalidSprintProcess
		}
		ids[id] = struct{}{}

		wip := stage.WipLimit
		if wip < 0 {
			wip = 0
		}
		workflows := stage.Workflows
		if workflows == nil {
			workflows = []StageWorkflowBinding{}
		}
		out = append(out, SprintStage{
			ID:          id,
			Key:         key,
			Name:        name,
			SortOrder:   i,
			Color:       strings.TrimSpace(stage.Color),
			WipLimit:    wip,
			EntryPolicy: stage.EntryPolicy,
			Workflows:   workflows,
		})
	}
	return out, nil
}

type WorkItemStageMove struct {
	WorkItemID kernel.WorkItemID
	NewStageID string
}

// PlanSprintStageUpdate validates a stage layout change and plans work item migrations.
func PlanSprintStageUpdate(
	current []SprintStage,
	next []SprintStage,
	items []*WorkItem,
	migrations map[string]string,
) ([]SprintStage, []WorkItemStageMove, error) {
	normalized, err := NormalizeSprintStages(next)
	if err != nil {
		return nil, nil, err
	}
	newIDs := map[string]struct{}{}
	for _, stage := range normalized {
		newIDs[stage.ID] = struct{}{}
	}
	removed := map[string]struct{}{}
	for _, stage := range current {
		if _, ok := newIDs[stage.ID]; !ok {
			removed[stage.ID] = struct{}{}
		}
	}
	moves := make([]WorkItemStageMove, 0)
	for _, item := range items {
		if _, ok := removed[item.StageID]; !ok {
			continue
		}
		target := strings.TrimSpace(migrations[item.StageID])
		if target == "" {
			return nil, nil, domain.ErrStageHasWorkItems
		}
		if _, ok := newIDs[target]; !ok {
			return nil, nil, domain.ErrStageHasWorkItems
		}
		moves = append(moves, WorkItemStageMove{WorkItemID: item.ID, NewStageID: target})
	}
	return normalized, moves, nil
}

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
		ID:          kernel.NewWorkspaceSprintProcessTemplateID(),
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(name),
		TemplateKey: strings.TrimSpace(templateKey),
		Description: strings.TrimSpace(description),
		Stages:      normalized,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// DefaultSprintProcessTemplates returns built-in stage presets for UI seeding.
func DefaultSprintProcessTemplates() map[string][]SprintStage {
	return map[string][]SprintStage{
		"agile-4": DefaultSprintStages(),
		"lean-3": {
			{ID: uuid.NewString(), Key: "todo", Name: "待办", SortOrder: 0, Color: "#64748b"},
			{ID: uuid.NewString(), Key: "doing", Name: "进行中", SortOrder: 1, Color: "#3b82f6"},
			{ID: uuid.NewString(), Key: "done", Name: "完成", SortOrder: 2, Color: "#10b981"},
		},
		"devops-6": {
			{ID: uuid.NewString(), Key: "backlog", Name: "需求", SortOrder: 0, Color: "#64748b"},
			{ID: uuid.NewString(), Key: "dev", Name: "开发", SortOrder: 1, Color: "#3b82f6"},
			{ID: uuid.NewString(), Key: "ci", Name: "CI", SortOrder: 2, Color: "#06b6d4"},
			{ID: uuid.NewString(), Key: "test", Name: "测试", SortOrder: 3, Color: "#8b5cf6"},
			{ID: uuid.NewString(), Key: "uat", Name: "预发", SortOrder: 4, Color: "#f59e0b"},
			{ID: uuid.NewString(), Key: "prod", Name: "生产", SortOrder: 5, Color: "#10b981"},
		},
	}
}
