package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type KnowledgeBaseStatus string

const (
	KnowledgeBaseStatusActive   KnowledgeBaseStatus = "active"
	KnowledgeBaseStatusArchived KnowledgeBaseStatus = "archived"
)

type KnowledgeBaseName struct {
	value string
}

func NewKnowledgeBaseName(raw string) (KnowledgeBaseName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return KnowledgeBaseName{}, domain.ErrInvalidKnowledgeBaseName
	}
	return KnowledgeBaseName{value: v}, nil
}

func (n KnowledgeBaseName) String() string { return n.value }

type KnowledgeBase struct {
	ID          kernel.KnowledgeBaseID
	WorkspaceID kernel.WorkspaceID
	Name        KnowledgeBaseName
	Description string
	Status      KnowledgeBaseStatus
	CreatedAt   time.Time
}

func NewKnowledgeBase(wsID kernel.WorkspaceID, name KnowledgeBaseName, description string) *KnowledgeBase {
	return &KnowledgeBase{
		ID:          kernel.NewKnowledgeBaseID(),
		WorkspaceID: wsID,
		Name:        name,
		Description: strings.TrimSpace(description),
		Status:      KnowledgeBaseStatusActive,
		CreatedAt:   time.Now().UTC(),
	}
}

func (kb *KnowledgeBase) Update(name KnowledgeBaseName, description string) error {
	if kb.Status == KnowledgeBaseStatusArchived {
		return domain.ErrKnowledgeBaseArchived
	}
	kb.Name = name
	kb.Description = strings.TrimSpace(description)
	return nil
}

func (kb *KnowledgeBase) Archive() error {
	if kb.Status == KnowledgeBaseStatusArchived {
		return domain.ErrKnowledgeBaseArchived
	}
	kb.Status = KnowledgeBaseStatusArchived
	return nil
}
