package query

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListFileAuditEvents struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	ResourceID     string
	Result         string
	From           *time.Time
	To             *time.Time
	Cursor         string
	Limit          int
}

type FileAuditEvent struct {
	ID           string         `json:"id"`
	ActorID      string         `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Result       string         `json:"result"`
	TraceID      string         `json:"trace_id,omitempty"`
	CreatedAt    string         `json:"created_at"`
	Detail       map[string]any `json:"detail,omitempty"`
}

type ListFileAuditEventsResult struct {
	Items      []FileAuditEvent
	NextCursor string
}

type auditLister interface {
	List(ctx context.Context, filter audit.ListFilter) ([]audit.LogView, error)
}

type ListFileAuditEventsHandler struct {
	audit auditLister
}

func NewListFileAuditEventsHandler(store auditLister) *ListFileAuditEventsHandler {
	if store == nil {
		panic("filesystem audit list requires audit store")
	}
	return &ListFileAuditEventsHandler{audit: store}
}

func (h *ListFileAuditEventsHandler) Handle(ctx context.Context, q ListFileAuditEvents) (ListFileAuditEventsResult, error) {
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if q.Cursor != "" {
		parsed, err := strconv.Atoi(q.Cursor)
		if err != nil || parsed < 0 {
			return ListFileAuditEventsResult{}, fmt.Errorf("invalid audit cursor")
		}
		offset = parsed
	}
	filter := audit.ListFilter{
		OrgID:        &kernel.OrgID{UUID: q.OrganizationID},
		Limit:        limit,
		Offset:       offset,
		ActionPrefix: "filesystem.",
		ResourceType: "filesystem",
		WorkspaceID:  q.WorkspaceID.String(),
	}
	if q.Action != "" {
		filter.ActionPrefix = ""
		filter.Action = strings.TrimSpace(q.Action)
	}
	if q.ResourceID != "" {
		filter.ResourceID = strings.TrimSpace(q.ResourceID)
	}
	if q.ActorID != nil {
		filter.ActorID = q.ActorID.String()
	}
	if q.Result != "" {
		filter.Decision = strings.ToUpper(strings.TrimSpace(q.Result))
	}
	if q.From != nil {
		filter.CreatedAtAfter = q.From.UTC()
	}
	if q.To != nil {
		filter.CreatedAtBefore = q.To.UTC()
	}
	rows, err := h.audit.List(ctx, filter)
	if err != nil {
		return ListFileAuditEventsResult{}, err
	}
	items := make([]FileAuditEvent, 0, len(rows))
	for _, row := range rows {
		result := "ALLOW"
		traceID := ""
		if row.Detail != nil {
			if decision, ok := row.Detail["decision"].(string); ok && decision != "" {
				result = decision
			}
			if trace, ok := row.Detail["trace_id"].(string); ok {
				traceID = trace
			}
		}
		items = append(items, FileAuditEvent{
			ID: row.ID, ActorID: row.ActorID, Action: row.Action,
			ResourceType: row.ResourceType, ResourceID: row.ResourceID,
			Result: result, TraceID: traceID, CreatedAt: row.CreatedAt, Detail: row.Detail,
		})
	}
	next := ""
	if len(rows) == limit {
		next = strconv.Itoa(offset + limit)
	}
	return ListFileAuditEventsResult{Items: items, NextCursor: next}, nil
}
