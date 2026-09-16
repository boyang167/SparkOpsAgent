package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TemplateScope string

const (
	TemplateScopeSystem    TemplateScope = "system"
	TemplateScopeWorkspace TemplateScope = "workspace"
)

type TemplateStatus string

const (
	TemplateStatusDraft   TemplateStatus = "draft"
	TemplateStatusActive  TemplateStatus = "active"
	TemplateStatusInactive TemplateStatus = "inactive"
)

type KeyVal struct {
	Key           string   `json:"key"`
	Value         string   `json:"value,omitempty"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type,omitempty"`
	ChoiceOptions []string `json:"choice_options,omitempty"`
	Required      bool     `json:"required,omitempty"`
	Source        string   `json:"source,omitempty"`
}

type WorkflowTemplate struct {
	ID           kernel.WorkflowTemplateID
	Scope        TemplateScope
	WorkspaceID  *kernel.WorkspaceID
	Name         string
	Key          string
	Description  string
	Category     string
	ProductTypes []string
	Status       TemplateStatus
	Version      string
	Definition   json.RawMessage
	SlotSchema   json.RawMessage
	KeyVals      []KeyVal
	Runtime      json.RawMessage
	CreatedBy    kernel.MemberID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewWorkspaceTemplate(wsID kernel.WorkspaceID, name, key, description string, definition json.RawMessage, createdBy kernel.MemberID) *WorkflowTemplate {
	now := time.Now().UTC()
	return &WorkflowTemplate{
		ID: kernel.NewWorkflowTemplateID(), Scope: TemplateScopeWorkspace, WorkspaceID: &wsID,
		Name: name, Key: key, Description: description, Status: TemplateStatusDraft, Version: "0.1.0",
		Definition: definition, SlotSchema: json.RawMessage(`{}`), KeyVals: []KeyVal{},
		Runtime: json.RawMessage(`{}`), CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
}

func (t *WorkflowTemplate) EnsureWorkspaceEditable(wsID kernel.WorkspaceID) error {
	if t.Scope != TemplateScopeWorkspace {
		return domain.ErrWorkflowTemplateReadonly
	}
	if t.WorkspaceID == nil || *t.WorkspaceID != wsID {
		return domain.ErrWorkflowTemplateNotFound
	}
	return nil
}

func (t *WorkflowTemplate) UpdateContent(name, description string, definition json.RawMessage) {
	if name != "" {
		t.Name = name
	}
	t.Description = description
	if len(definition) > 0 {
		t.Definition = definition
	}
	t.UpdatedAt = time.Now().UTC()
}

func (t *WorkflowTemplate) Publish() error {
	if t.Status != TemplateStatusDraft {
		return domain.ErrInvalidWorkflowTemplateTransition
	}
	t.Status = TemplateStatusActive
	t.Version = bumpPatchVersion(t.Version)
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func bumpPatchVersion(version string) string {
	if version == "" {
		return "1.0.0"
	}
	var major, minor, patch int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return "1.0.0"
	}
	patch++
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}
