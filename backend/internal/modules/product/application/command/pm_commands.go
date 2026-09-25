package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ImportPmSprintInput struct {
	Provider      string
	PmWorkspaceID string
	IterationID   string
	TargetStageID string
	IssueTypes    []string
	SyncMode      string
}

type ImportPmSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	pm       *port.PmConnectorRouter
	board    *service.SprintBoardNotifier
}

func NewImportPmSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository, pm *port.PmConnectorRouter, board *service.SprintBoardNotifier) *ImportPmSprintHandler {
	return &ImportPmSprintHandler{products: products, sprints: sprints, pm: pm, board: board}
}

func (h *ImportPmSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID, actor kernel.MemberID, input ImportPmSprintInput) (*ImportJiraSprintResult, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = port.PmProviderJira
	}
	connector := h.pm.Get(provider)
	if connector == nil {
		return nil, domain.ErrPmNotConfigured
	}
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
	issues, err := connector.GetIterationIssues(ctx, product.WorkspaceID, input.PmWorkspaceID, input.IterationID, input.IssueTypes)
	if err != nil {
		return nil, err
	}
	existing, err := h.sprints.ListWorkItemsBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	linked := map[string]struct{}{}
	for _, item := range existing {
		if key := model.WorkItemPmIssueKey(item); key != "" {
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
		item.ExternalRef = model.BuildPmExternalRef(provider, issue, syncMode, input.PmWorkspaceID)
		item.SortOrder = maxOrder
		if err := h.sprints.SaveWorkItem(ctx, item); err != nil {
			return nil, err
		}
		linked[key] = struct{}{}
		result.ImportedCount++
		result.WorkItemIDs = append(result.WorkItemIDs, item.ID)
	}
	if result.ImportedCount > 0 && h.board != nil {
		h.board.BoardChanged(sprintID, "imported")
	}
	return result, nil
}

type LinkPmIssueInput struct {
	Provider      string
	PmWorkspaceID string
	IssueKey      string
	SyncMode      string
}

type LinkPmIssueHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	pm       *port.PmConnectorRouter
}

func NewLinkPmIssueHandler(products repository.ProductRepository, sprints repository.SprintRepository, pm *port.PmConnectorRouter) *LinkPmIssueHandler {
	return &LinkPmIssueHandler{products: products, sprints: sprints, pm: pm}
}

func (h *LinkPmIssueHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID, input LinkPmIssueInput) (*model.WorkItem, error) {
	item, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != model.SprintStatusPlanning && sprint.Status != model.SprintStatusActive {
		return nil, domain.ErrInvalidSprintTransition
	}
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = port.PmProviderJira
	}
	issueKey := strings.TrimSpace(input.IssueKey)
	if issueKey == "" {
		return nil, domain.ErrInvalidWorkItemInput
	}
	if provider == port.PmProviderJira {
		issueKey = strings.ToUpper(issueKey)
	}
	existing, err := h.sprints.ListWorkItemsBySprint(ctx, sprint.ID)
	if err != nil {
		return nil, err
	}
	for _, other := range existing {
		if other.ID == item.ID {
			continue
		}
		if strings.EqualFold(model.WorkItemPmIssueKey(other), issueKey) {
			return nil, domain.ErrPmIssueAlreadyLinked
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
	var issue port.PmIssue
	if connector := h.pm.Get(provider); connector != nil {
		fetched, err := connector.GetIssue(ctx, product.WorkspaceID, input.PmWorkspaceID, issueKey)
		if err == nil && fetched != nil {
			issue = *fetched
		} else {
			issue = port.PmIssue{Key: issueKey}
		}
	} else {
		issue = port.PmIssue{Key: issueKey}
	}
	item.ExternalRef = model.BuildPmExternalRef(provider, issue, syncMode, input.PmWorkspaceID)
	if issue.Summary != "" && strings.TrimSpace(item.Title) == "" {
		item.Title = issue.Summary
	}
	if err := h.sprints.UpdateWorkItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
