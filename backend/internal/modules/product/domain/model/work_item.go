package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkItemType string

const (
	WorkItemTypeStory   WorkItemType = "story"
	WorkItemTypeTask    WorkItemType = "task"
	WorkItemTypeBug     WorkItemType = "bug"
	WorkItemTypeEpic    WorkItemType = "epic"
	WorkItemTypeRelease WorkItemType = "release"
	WorkItemTypeCustom  WorkItemType = "custom"
)

type WorkItemPriority string

const (
	WorkItemPriorityCritical WorkItemPriority = "critical"
	WorkItemPriorityHigh     WorkItemPriority = "high"
	WorkItemPriorityMedium   WorkItemPriority = "medium"
	WorkItemPriorityLow      WorkItemPriority = "low"
)

type WorkItemStatus string

const (
	WorkItemStatusOpen       WorkItemStatus = "open"
	WorkItemStatusInProgress WorkItemStatus = "in_progress"
	WorkItemStatusBlocked    WorkItemStatus = "blocked"
	WorkItemStatusDone       WorkItemStatus = "done"
	WorkItemStatusCancelled  WorkItemStatus = "cancelled"
)

type WorkItem struct {
	ID            kernel.WorkItemID
	SprintID      kernel.SprintID
	StageID       string
	ParentID      *kernel.WorkItemID
	SortOrder     int
	Type          WorkItemType
	Title         string
	Description   string
	Priority      WorkItemPriority
	Status        WorkItemStatus
	AssigneeID    *kernel.MemberID
	ReporterID    *kernel.MemberID
	ProjectIDs    []string
	Labels        []string
	ExternalRef   map[string]any
	Delivery      map[string]any
	BlockedReason string
	Version       int
	CreatedBy     kernel.MemberID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewWorkItem(sprintID kernel.SprintID, stageID, title string, itemType WorkItemType, priority WorkItemPriority, createdBy kernel.MemberID) *WorkItem {
	now := time.Now().UTC()
	if itemType == "" {
		itemType = WorkItemTypeStory
	}
	if priority == "" {
		priority = WorkItemPriorityMedium
	}
	return &WorkItem{
		ID:        kernel.NewWorkItemID(),
		SprintID:  sprintID,
		StageID:   stageID,
		Type:      itemType,
		Title:     strings.TrimSpace(title),
		Priority:  priority,
		Status:    WorkItemStatusOpen,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
