package model

import (
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func CheckStageEntry(
	sprint *Sprint,
	item *WorkItem,
	targetStageID string,
	allItems []*WorkItem,
	executions []*WorkItemExecution,
) error {
	stage := sprint.StageByID(targetStageID)
	if stage == nil {
		return domain.ErrInvalidStage
	}
	if item.StageID != targetStageID && stage.WipLimit > 0 {
		count := 0
		for _, wi := range allItems {
			if wi.StageID == targetStageID {
				count++
			}
		}
		if count >= stage.WipLimit {
			return domain.ErrStageGateDenied
		}
	}
	if stage.EntryPolicy == nil {
		return nil
	}
	policy := stage.EntryPolicy
	if policy.RequireBranch {
		branch, _ := item.Delivery["branch"].(string)
		if strings.TrimSpace(branch) == "" {
			return domain.ErrStageGateDenied
		}
	}
	for _, wfID := range policy.RequireWorkflowSuccess {
		if !hasSuccessfulWorkflowExecution(executions, wfID) {
			return domain.ErrStageGateDenied
		}
	}
	if len(policy.RequireJiraStatus) > 0 && WorkItemPmIssueKey(item) != "" {
		status, _ := item.ExternalRef["pm_status"].(string)
		if status == "" {
			status, _ = item.ExternalRef["jira_status"].(string)
		}
		if !jiraStatusAllowed(status, policy.RequireJiraStatus) {
			return domain.ErrStageGateDenied
		}
	}
	return nil
}

func hasSuccessfulWorkflowExecution(execs []*WorkItemExecution, workflowID string) bool {
	wfID, err := kernel.ParseProductWorkflowID(workflowID)
	if err != nil {
		return false
	}
	for _, exec := range execs {
		if exec.WorkflowID == wfID && exec.Status == WorkItemExecutionSuccess {
			return true
		}
	}
	return false
}

func jiraStatusAllowed(status string, allowed []string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	for _, raw := range allowed {
		if strings.ToLower(strings.TrimSpace(raw)) == status {
			return true
		}
	}
	return false
}
