package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
}

func NewTemplate(wsID kernel.WorkspaceID, name TemplateName, kind, language, detail, rules string, createdBy kernel.MemberID) *Template {
	return &Template{
		ID:          kernel.NewTemplateID(),
		WorkspaceID: wsID,
		Name:        name,
		Kind:        strings.TrimSpace(kind),
		Language:    strings.TrimSpace(language),
		Detail:      detail,
		Rules:       rules,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}
}
