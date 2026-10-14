package model

import (
	"reflect"
	"strings"
	"time"

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

// ApplyStageWorkflowBindings returns current stages with workflow bindings taken from next.
// ok is false when next changes anything other than workflow bindings.
func ApplyStageWorkflowBindings(current, next []SprintStage) ([]SprintStage, bool) {
	if len(current) != len(next) {
		return nil, false
	}
	byID := make(map[string]SprintStage, len(next))
	for _, st := range next {
		byID[st.ID] = st
	}
	out := make([]SprintStage, len(current))
	for i, cur := range current {
		nxt, found := byID[cur.ID]
		if !found {
			return nil, false
		}
		a, b := cur, nxt
		a.Workflows, b.Workflows = nil, nil
		if !reflect.DeepEqual(a, b) {
			return nil, false
		}
		cur.Workflows = NormalizeStageWorkflowBindings(nxt.Workflows)
		out[i] = cur
	}
	return out, true
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
