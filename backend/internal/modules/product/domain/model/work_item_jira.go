package model

import (
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
)

func WorkItemPmIssueKey(item *WorkItem) string {
	if item == nil || item.ExternalRef == nil {
		return ""
	}
	if key, _ := item.ExternalRef["issue_key"].(string); key != "" {
		return key
	}
	return ""
}

func WorkItemJiraIssueKey(item *WorkItem) string { return WorkItemPmIssueKey(item) }

func WorkItemJiraSyncMode(item *WorkItem) string {
	if item == nil || item.ExternalRef == nil {
		return "link_only"
	}
	if mode, _ := item.ExternalRef["sync_mode"].(string); mode != "" {
		return mode
	}
	return "link_only"
}

func BuildPmExternalRef(provider string, issue port.PmIssue, syncMode, pmWorkspaceID string) map[string]any {
	if syncMode == "" {
		syncMode = "link_only"
	}
	if provider == "" {
		provider = port.PmProviderJira
	}
	ref := map[string]any{
		"provider":  provider,
		"issue_key": issue.Key,
		"sync_mode": syncMode,
	}
	if issue.ID != "" {
		ref["issue_id"] = issue.ID
	}
	if issue.IssueType != "" {
		ref["issue_type"] = issue.IssueType
	}
	if issue.URL != "" {
		ref["issue_url"] = issue.URL
	}
	if issue.Status != "" {
		ref["pm_status"] = issue.Status
		ref["jira_status"] = issue.Status
	}
	if pmWorkspaceID != "" {
		ref["pm_workspace_id"] = pmWorkspaceID
	}
	return ref
}

func BuildJiraExternalRef(issue port.JiraIssue, syncMode string) map[string]any {
	return BuildPmExternalRef(port.PmProviderJira, port.PmIssue{
		Key: issue.Key, ID: issue.ID, IssueType: issue.IssueType, URL: issue.URL, Status: issue.Status,
	}, syncMode, "")
}

func WorkItemPmProvider(item *WorkItem) string {
	if item == nil || item.ExternalRef == nil {
		return ""
	}
	if p, _ := item.ExternalRef["provider"].(string); p != "" {
		return p
	}
	return port.PmProviderJira
}

func WorkItemPmWorkspaceID(item *WorkItem) string {
	if item == nil || item.ExternalRef == nil {
		return ""
	}
	if v, _ := item.ExternalRef["pm_workspace_id"].(string); v != "" {
		return v
	}
	return ""
}

func MapJiraIssueType(name string) WorkItemType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bug":
		return WorkItemTypeBug
	case "task", "sub-task", "subtask":
		return WorkItemTypeTask
	case "epic":
		return WorkItemTypeEpic
	case "story", "user story":
		return WorkItemTypeStory
	default:
		return WorkItemTypeStory
	}
}

func MapJiraPriority(name string) WorkItemPriority {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "highest", "blocker", "critical":
		return WorkItemPriorityCritical
	case "high", "major":
		return WorkItemPriorityHigh
	case "low", "minor", "trivial":
		return WorkItemPriorityLow
	default:
		return WorkItemPriorityMedium
	}
}
