package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TemplateStatus string

const (
	TemplateStatusActive   TemplateStatus = "active"
	TemplateStatusInactive TemplateStatus = "inactive"
	TemplateStatusDraft    TemplateStatus = "draft"
)

type TemplateName struct {
	value string
}

func NewTemplateName(raw string) (TemplateName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return TemplateName{}, domain.ErrInvalidTemplateName
	}
	return TemplateName{value: v}, nil
}

func (n TemplateName) String() string { return n.value }

type Template struct {
	ID          kernel.TemplateID
	WorkspaceID kernel.WorkspaceID
	Name        TemplateName
	Kind        string
	Language    string
	Detail      string
	Rules       string
	Status      TemplateStatus
	IsPublic    bool
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewTemplate(
	wsID kernel.WorkspaceID,
	name TemplateName,
	kind, language, detail, rules string,
	status TemplateStatus,
	isPublic bool,
	createdBy kernel.MemberID,
) *Template {
	now := time.Now().UTC()
	if status == "" {
		status = TemplateStatusActive
	}
	return &Template{
		ID:          kernel.NewTemplateID(),
		WorkspaceID: wsID,
		Name:        name,
		Kind:        strings.TrimSpace(kind),
		Language:    strings.TrimSpace(language),
		Detail:      detail,
		Rules:       rules,
		Status:      status,
		IsPublic:    isPublic,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (t *Template) Update(name TemplateName, kind, language, detail, rules string, status TemplateStatus, isPublic bool) {
	t.Name = name
	t.Kind = strings.TrimSpace(kind)
	t.Language = strings.TrimSpace(language)
	t.Detail = detail
	t.Rules = rules
	if status != "" {
		t.Status = status
	}
	t.IsPublic = isPublic
	t.UpdatedAt = time.Now().UTC()
}

// TemplateView enriches a template with source workspace metadata for cross-workspace listing.
type TemplateView struct {
	Template      *Template
	WorkspaceName string
}
