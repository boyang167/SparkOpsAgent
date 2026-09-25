package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ImportJiraSprintInput struct {
	JiraSprintID  string
	TargetStageID string
	IssueTypes    []string
	SyncMode      string
}

type ImportJiraSprintResult struct {
	SprintID       kernel.SprintID
	ImportedCount  int
	SkippedCount   int
	WorkItemIDs    []kernel.WorkItemID
}

type ImportJiraSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	jira     port.JiraConnector
}

func NewImportJiraSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository, jira port.JiraConnector) *ImportJiraSprintHandler {
	return &ImportJiraSprintHandler{products: products, sprints: sprints, jira: jira}
}

func (h *ImportJiraSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID, actor kernel.MemberID, input ImportJiraSprintInput) (*ImportJiraSprintResult, error) {
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != model.SprintStatusPlanning && sprint.Status != model.SprintStatusActive {
		return nil, domain.ErrInvalidSprintTransition
	}
	if h.jira == nil {
		return nil, domain.ErrJiraNotConfigured
	}
	issues, err := h.jira.GetSprintIssues(ctx, product.WorkspaceID, strings.TrimSpace(input.JiraSprintID), input.IssueTypes)
	if err != nil {
		return nil, err
	}
	existing, err := h.sprints.ListWorkItemsBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	linked := map[string]struct{}{}
	for _, item := range existing {
		if key := model.WorkItemJiraIssueKey(item); key != "" {
			linked[strings.ToUpper(key)] = struct{}{}
		}
	}
	stageID := strings.TrimSpace(input.TargetStageID)
	if stageID == "" {
		stageID = sprint.FirstStageID()
	}
	if stageID == "" {
		return nil, domain.ErrInvalidStage
	}
	syncMode := strings.TrimSpace(input.SyncMode)
	if syncMode == "" {
		syncMode = "link_only"
	}
	result := &ImportJiraSprintResult{SprintID: sprintID}
	maxOrder := 0
	for _, item := range existing {
		if item.StageID == stageID && item.SortOrder > maxOrder {
			maxOrder = item.SortOrder
		}
	}
	for _, issue := range issues {
		key := strings.ToUpper(strings.TrimSpace(issue.Key))
		if key == "" {
			continue
		}
		if _, ok := linked[key]; ok {
			result.SkippedCount++
			continue
		}
		maxOrder++
		item := model.NewWorkItem(sprintID, stageID, issue.Summary, model.MapJiraIssueType(issue.IssueType), model.MapJiraPriority(issue.Priority), actor)
		if issue.Description != "" {
			item.Description = issue.Description
		}
		item.ExternalRef = model.BuildJiraExternalRef(issue, syncMode)
		item.SortOrder = maxOrder
		if err := h.sprints.SaveWorkItem(ctx, item); err != nil {
			return nil, err
		}
		linked[key] = struct{}{}
		result.ImportedCount++
		result.WorkItemIDs = append(result.WorkItemIDs, item.ID)
	}
	return result, nil
}

type LinkJiraIssueInput struct {
	IssueKey string
	SyncMode string
}

type LinkJiraIssueHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	jira     port.JiraConnector
}

func NewLinkJiraIssueHandler(products repository.ProductRepository, sprints repository.SprintRepository, jira port.JiraConnector) *LinkJiraIssueHandler {
	return &LinkJiraIssueHandler{products: products, sprints: sprints, jira: jira}
}

func (h *LinkJiraIssueHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID, input LinkJiraIssueInput) (*model.WorkItem, error) {
	item, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != model.SprintStatusPlanning && sprint.Status != model.SprintStatusActive {
		return nil, domain.ErrInvalidSprintTransition
	}
	issueKey := strings.ToUpper(strings.TrimSpace(input.IssueKey))
	if issueKey == "" {
		return nil, domain.ErrInvalidWorkItemInput
	}
	existing, err := h.sprints.ListWorkItemsBySprint(ctx, sprint.ID)
	if err != nil {
		return nil, err
	}
	for _, other := range existing {
		if other.ID == item.ID {
			continue
		}
		if strings.EqualFold(model.WorkItemJiraIssueKey(other), issueKey) {
			return nil, domain.ErrJiraIssueAlreadyLinked
		}
	}
	syncMode := strings.TrimSpace(input.SyncMode)
	if syncMode == "" {
		syncMode = "link_only"
	}
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	var issue port.JiraIssue
	if h.jira != nil {
		fetched, err := h.jira.GetIssue(ctx, product.WorkspaceID, issueKey)
		if err == nil && fetched != nil {
			issue = *fetched
		} else {
			issue = port.JiraIssue{Key: issueKey}
		}
	} else {
		issue = port.JiraIssue{Key: issueKey}
	}
	item.ExternalRef = model.BuildJiraExternalRef(issue, syncMode)
	if issue.Summary != "" && strings.TrimSpace(item.Title) == "" {
		item.Title = issue.Summary
	}
	if err := h.sprints.UpdateWorkItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
