package query

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	NamespaceID    string
	NodeID         string
	PrincipalID    string
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
		NamespaceID:  strings.TrimSpace(q.NamespaceID),
		NodeID:       strings.TrimSpace(q.NodeID),
		PrincipalID:  strings.TrimSpace(q.PrincipalID),
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
		if filter.Decision == "ALLOW" {
			filter.AllowMissingDecisionAction = "filesystem.capability.issued"
		}
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
			Result: result, TraceID: traceID, CreatedAt: row.CreatedAt, Detail: sanitizeAuditDetail(row.Detail),
		})
	}
	next := ""
	if len(rows) == limit {
		next = strconv.Itoa(offset + limit)
	}
	return ListFileAuditEventsResult{Items: items, NextCursor: next}, nil
}

func sanitizeAuditDetail(detail map[string]any) map[string]any {
	if detail == nil {
		return map[string]any{}
	}
	sanitized := make(map[string]any, len(detail))
	for key, value := range detail {
		if sensitiveAuditKey(key) {
			sanitized[key] = "[REDACTED]"
			continue
		}
		sanitized[key] = sanitizeAuditValue(value)
	}
	return sanitized
}

func sanitizeAuditValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeAuditDetail(typed)
	case []any:
		sanitized := make([]any, len(typed))
		for index, item := range typed {
			sanitized[index] = sanitizeAuditValue(item)
		}
		return sanitized
	default:
		return value
	}
}

func sensitiveAuditKey(key string) bool {
	segments := strings.Split(normalizeAuditKey(key), "_")
	sensitivePatterns := [][]string{
		{"api", "key"},
		{"access", "key"},
		{"private", "key"},
		{"token"},
		{"password"},
		{"secret"},
		{"authorization"},
		{"cookie"},
		{"credential"},
		{"capability"},
	}
	allowedMetadataSuffixes := map[string]bool{
		"id": true, "count": true, "algorithm": true,
	}
	for start := range segments {
		for _, pattern := range sensitivePatterns {
			if !segmentsMatch(segments[start:], pattern) {
				continue
			}
			remaining := segments[start+len(pattern):]
			if len(remaining) == 1 && allowedMetadataSuffixes[remaining[0]] {
				continue
			}
			return true
		}
	}
	return false
}

func segmentsMatch(segments, pattern []string) bool {
	if len(segments) < len(pattern) {
		return false
	}
	for index := range pattern {
		if segments[index] != pattern[index] {
			return false
		}
	}
	return true
}

func normalizeAuditKey(key string) string {
	input := []rune(strings.TrimSpace(key))
	normalized := make([]rune, 0, len(input))
	appendSeparator := func() {
		if len(normalized) > 0 && normalized[len(normalized)-1] != '_' {
			normalized = append(normalized, '_')
		}
	}
	for index, current := range input {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			appendSeparator()
			continue
		}
		if unicode.IsUpper(current) && index > 0 {
			previous := input[index-1]
			nextIsLower := index+1 < len(input) && unicode.IsLower(input[index+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				(unicode.IsUpper(previous) && nextIsLower) {
				appendSeparator()
			}
		}
		normalized = append(normalized, unicode.ToLower(current))
	}
	return strings.Trim(string(normalized), "_")
}
