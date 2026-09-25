package model

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintStatus string

const (
	SprintStatusPlanning  SprintStatus = "planning"
	SprintStatusActive    SprintStatus = "active"
	SprintStatusCompleted SprintStatus = "completed"
	SprintStatusArchived  SprintStatus = "archived"
)

type StageWorkflowBinding struct {
	WorkflowID         string            `json:"workflow_id"`
	WorkflowName       string            `json:"workflow_name,omitempty"`
	TriggerMode        string            `json:"trigger_mode"`
	DefaultParameters  map[string]string `json:"default_parameters,omitempty"`
	RequiredForExit    bool              `json:"required_for_exit,omitempty"`
}

type StageGatePolicy struct {
	RequireBranch          bool     `json:"require_branch,omitempty"`
	RequireWorkflowSuccess []string `json:"require_workflow_success,omitempty"`
	RequireJiraStatus      []string `json:"require_jira_status,omitempty"`
	RequireApproval        bool     `json:"require_approval,omitempty"`
	ProtectedBranches      []string `json:"protected_branches,omitempty"`
}

type SprintStage struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Name        string                 `json:"name"`
	SortOrder   int                    `json:"sort_order"`
	Color       string                 `json:"color,omitempty"`
	WipLimit    int                    `json:"wip_limit,omitempty"`
	EntryPolicy *StageGatePolicy       `json:"entry_policy,omitempty"`
	Workflows   []StageWorkflowBinding `json:"workflows,omitempty"`
}

type SprintProcess struct {
	ID          kernel.SprintProcessID
	ProductID   kernel.ProductID
	Name        string
	TemplateKey string
	Description string
	Version     int
	IsDefault   bool
	Stages      []SprintStage
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Sprint struct {
	ID             kernel.SprintID
	ProductID      kernel.ProductID
	ProcessID      kernel.SprintProcessID
	ProcessVersion int
	Stages         []SprintStage
	Name           string
	Goal           string
	Status         SprintStatus
	PlannedStart   *time.Time
	PlannedEnd     *time.Time
	ActualStart    *time.Time
	ActualEnd      *time.Time
	OwnerID        *kernel.MemberID
	ExternalRef    map[string]any
	LegacyPlanID   *kernel.ProductPlanID
	CreatedBy      kernel.MemberID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func DefaultSprintStages() []SprintStage {
	return []SprintStage{
		{ID: uuid.NewString(), Key: "dev", Name: "开发", SortOrder: 0, Color: "#3b82f6"},
		{ID: uuid.NewString(), Key: "test", Name: "测试", SortOrder: 1, Color: "#8b5cf6"},
		{ID: uuid.NewString(), Key: "uat", Name: "预发", SortOrder: 2, Color: "#f59e0b"},
		{ID: uuid.NewString(), Key: "release", Name: "发布", SortOrder: 3, Color: "#10b981"},
	}
}

func NewSprintProcess(productID kernel.ProductID, name string, stages []SprintStage, createdBy kernel.MemberID) *SprintProcess {
	if len(stages) == 0 {
		stages = DefaultSprintStages()
	}
	now := time.Now().UTC()
	return &SprintProcess{
		ID:        kernel.NewSprintProcessID(),
		ProductID: productID,
		Name:      strings.TrimSpace(name),
		Version:   1,
		IsDefault: true,
		Stages:    stages,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewSprint(productID kernel.ProductID, process *SprintProcess, name, goal string, ownerID *kernel.MemberID, createdBy kernel.MemberID) *Sprint {
	now := time.Now().UTC()
	stages := append([]SprintStage(nil), process.Stages...)
	return &Sprint{
		ID:             kernel.NewSprintID(),
		ProductID:      productID,
		ProcessID:      process.ID,
		ProcessVersion: process.Version,
		Stages:         stages,
		Name:           strings.TrimSpace(name),
		Goal:           strings.TrimSpace(goal),
		Status:         SprintStatusPlanning,
		OwnerID:        ownerID,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s *Sprint) Start(now time.Time) error {
	if s.Status != SprintStatusPlanning {
		return sprintTransitionErr
	}
	s.Status = SprintStatusActive
	s.ActualStart = &now
	s.UpdatedAt = now
	return nil
}

func (s *Sprint) Complete(now time.Time) error {
	if s.Status != SprintStatusActive {
		return sprintTransitionErr
	}
	s.Status = SprintStatusCompleted
	s.ActualEnd = &now
	s.UpdatedAt = now
	return nil
}

func (s *Sprint) Archive(now time.Time) error {
	if s.Status != SprintStatusCompleted {
		return sprintTransitionErr
	}
	s.Status = SprintStatusArchived
	s.UpdatedAt = now
	return nil
}

func (s *Sprint) FirstStageID() string {
	if len(s.Stages) == 0 {
		return ""
	}
	min := s.Stages[0]
	for _, st := range s.Stages[1:] {
		if st.SortOrder < min.SortOrder {
			min = st
		}
	}
	return min.ID
}

func (s *Sprint) HasStage(stageID string) bool {
	return s.StageByID(stageID) != nil
}

func (s *Sprint) StageByID(stageID string) *SprintStage {
	for i := range s.Stages {
		if s.Stages[i].ID == stageID {
			return &s.Stages[i]
		}
	}
	return nil
}

func (s *Sprint) ResolveStageWorkflow(stageID, workflowID string) (*StageWorkflowBinding, error) {
	for _, st := range s.Stages {
		if st.ID != stageID {
			continue
		}
		if workflowID != "" {
			for i := range st.Workflows {
				if st.Workflows[i].WorkflowID == workflowID {
					return &st.Workflows[i], nil
				}
			}
			return nil, domain.ErrWorkflowNotConfigured
		}
		if len(st.Workflows) > 0 {
			return &st.Workflows[0], nil
		}
		return nil, domain.ErrWorkflowNotConfigured
	}
	return nil, domain.ErrInvalidStage
}

// sprintTransitionErr is compared in command layer via errors.Is against domain.ErrInvalidSprintTransition.
var sprintTransitionErr = errInvalidSprintTransition{}

type errInvalidSprintTransition struct{}

func (errInvalidSprintTransition) Error() string { return "invalid sprint status transition" }

func SprintTransitionError() error { return sprintTransitionErr }
