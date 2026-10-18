package application

import (
	"context"
	"fmt"

	productcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	productquery "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	pwfquery "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ActionDeps struct {
	ListProducts    *productquery.ListProductsHandler
	ListSprints     *productquery.ListSprintsHandler
	ListWorkflows   *pwfquery.ListProductWorkflowsHandler
	WorkflowTrigger appctx.WorkflowRunTrigger
	CreateWorkItem  *productcommand.CreateWorkItemHandler
}

func RegisterActionTools(registry *domain.Registry, deps ActionDeps) {
	deliveryModule := []string{"delivery"}

	registry.Register(domain.ToolSpec{
		Name:        "list_products",
		Description: "List products in the current workspace.",
		InputSchema: domain.ObjectSchema(map[string]any{}),
		Permission:  authz.PermProjectRead,
		Modules:     deliveryModule,
		Tier:        domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, _ map[string]any) (any, error) {
		items, err := deps.ListProducts.Handle(ctx, wsID, memberID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "list_product_workflows",
		Description: "List workflows configured for a product.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"product_id": domain.StringProperty("Product UUID"),
		}, "product_id"),
		Permission: authz.PermProjectRead,
		Modules:    deliveryModule,
		Tier:       domain.ToolTierDiscovery,
	}, func(ctx context.Context, wsID kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
		productID, err := parseProductIDArg(args)
		if err != nil {
			return nil, err
		}
		items, err := deps.ListWorkflows.Handle(ctx, wsID, productID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "list_sprints",
		Description: "List sprints for a product.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"product_id": domain.StringProperty("Product UUID"),
			"status":     domain.StringProperty("Optional sprint status filter"),
		}, "product_id"),
		Permission: authz.PermProjectRead,
		Modules:    deliveryModule,
		Tier:       domain.ToolTierDiscovery,
	}, func(ctx context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, args map[string]any) (any, error) {
		productID, err := parseProductIDArg(args)
		if err != nil {
			return nil, err
		}
		status, _ := args["status"].(string)
		items, err := deps.ListSprints.Handle(ctx, productID, status)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "trigger_workflow",
		Description: "Start a product workflow run.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"product_id":  domain.StringProperty("Product UUID"),
			"workflow_id": domain.StringProperty("Workflow UUID"),
			"input":       map[string]any{"type": "object", "description": "Optional workflow input variables"},
		}, "product_id", "workflow_id"),
		Permission: authz.PermProjectWrite,
		Modules:    deliveryModule,
		Tier:       domain.ToolTierAction,
	}, func(ctx context.Context, _ kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		if deps.WorkflowTrigger == nil {
			return nil, fmt.Errorf("workflow trigger unavailable")
		}
		productID, err := parseProductIDArg(args)
		if err != nil {
			return nil, err
		}
		workflowIDRaw, err := parseStringArg(args, "workflow_id")
		if err != nil {
			return nil, err
		}
		workflowID, err := kernel.ParseProductWorkflowID(workflowIDRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid workflow_id")
		}
		var input map[string]any
		if raw, ok := args["input"].(map[string]any); ok {
			input = raw
		}
		runID, err := deps.WorkflowTrigger.StartForProductWithInput(ctx, productID, workflowID, memberID, input)
		if err != nil {
			return nil, err
		}
		return map[string]any{"run_id": runID.String(), "status": "started"}, nil
	})

	registry.Register(domain.ToolSpec{
		Name:        "create_work_item",
		Description: "Create a work item in a product sprint.",
		InputSchema: domain.ObjectSchema(map[string]any{
			"product_id":  domain.StringProperty("Product UUID"),
			"sprint_id":   domain.StringProperty("Sprint UUID"),
			"title":       domain.StringProperty("Work item title"),
			"type":        domain.StringProperty("Optional type: story, task, bug, epic, release, custom"),
			"priority":    domain.StringProperty("Optional priority: critical, high, medium, low"),
			"description": domain.StringProperty("Optional description"),
			"stage_id":    domain.StringProperty("Optional stage id; defaults to first stage"),
		}, "product_id", "sprint_id", "title"),
		Permission: authz.PermProjectWrite,
		Modules:    deliveryModule,
		Tier:       domain.ToolTierAction,
	}, func(ctx context.Context, _ kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		productID, err := parseProductIDArg(args)
		if err != nil {
			return nil, err
		}
		sprintIDRaw, err := parseStringArg(args, "sprint_id")
		if err != nil {
			return nil, err
		}
		sprintID, err := kernel.ParseSprintID(sprintIDRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid sprint_id")
		}
		title, err := parseStringArg(args, "title")
		if err != nil {
			return nil, err
		}
		itemType := model.WorkItemTypeStory
		if raw, ok := args["type"].(string); ok && raw != "" {
			itemType = model.WorkItemType(raw)
		}
		priority := model.WorkItemPriorityMedium
		if raw, ok := args["priority"].(string); ok && raw != "" {
			priority = model.WorkItemPriority(raw)
		}
		description, _ := args["description"].(string)
		stageID, _ := args["stage_id"].(string)
		item, err := deps.CreateWorkItem.Handle(ctx, productID, sprintID, memberID, productcommand.CreateWorkItemInput{
			StageID: stageID, Type: itemType, Title: title, Description: description, Priority: priority,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":        item.ID.String(),
			"sprint_id": item.SprintID.String(),
			"title":     item.Title,
			"type":      string(item.Type),
			"status":    string(item.Status),
		}, nil
	})
}

func parseProductIDArg(args map[string]any) (kernel.ProductID, error) {
	raw, err := parseStringArg(args, "product_id")
	if err != nil {
		return kernel.ProductID{}, err
	}
	id, err := kernel.ParseProductID(raw)
	if err != nil {
		return kernel.ProductID{}, fmt.Errorf("invalid product_id")
	}
	return id, nil
}
