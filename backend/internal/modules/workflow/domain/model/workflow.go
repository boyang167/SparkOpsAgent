package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

type WorkflowName struct {
	value string
}

func NewWorkflowName(raw string) (WorkflowName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return WorkflowName{}, domain.ErrInvalidWorkflowName
	}
	return WorkflowName{value: v}, nil
}

func (n WorkflowName) String() string { return n.value }

type Workflow struct {
	ID          kernel.WorkflowID
	ProjectID   kernel.ProjectID
	Name        WorkflowName
	Description string
	Status      WorkflowStatus
	Definition  string
	CreatedAt   time.Time
}

func NewWorkflow(projectID kernel.ProjectID, name WorkflowName, description, definition string) *Workflow {
	if strings.TrimSpace(definition) == "" {
		definition = "{}"
	}
	return &Workflow{
		ID:          kernel.NewWorkflowID(),
		ProjectID:   projectID,
		Name:        name,
		Description: strings.TrimSpace(description),
		Status:      WorkflowStatusDraft,
		Definition:  definition,
		CreatedAt:   time.Now().UTC(),
	}
}

func (w *Workflow) Update(name WorkflowName, description, definition string) error {
	if w.Status == WorkflowStatusArchived {
		return domain.ErrWorkflowArchived
	}
	w.Name = name
	w.Description = strings.TrimSpace(description)
	if strings.TrimSpace(definition) != "" {
		w.Definition = definition
	}
	return nil
}

func (w *Workflow) Activate() error {
	if w.Status == WorkflowStatusArchived {
		return domain.ErrWorkflowArchived
	}
	w.Status = WorkflowStatusActive
	return nil
}

func (w *Workflow) Archive() error {
	if w.Status == WorkflowStatusArchived {
		return domain.ErrWorkflowArchived
	}
	w.Status = WorkflowStatusArchived
	return nil
}

func (w *Workflow) EnsureRunnable() error {
	if w.Status == WorkflowStatusArchived {
		return domain.ErrWorkflowNotFound
	}
	if w.Status != WorkflowStatusActive {
		return domain.ErrWorkflowNotActive
	}
	return nil
}
