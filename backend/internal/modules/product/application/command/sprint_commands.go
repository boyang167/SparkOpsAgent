package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateSprintInput struct {
	Name         string
	Goal         string
	PlannedStart *time.Time
	PlannedEnd   *time.Time
	OwnerID      *kernel.MemberID
	ProcessID    *kernel.SprintProcessID
}

type CreateSprintHandler struct {
	products  repository.ProductRepository
	sprints   repository.SprintRepository
	processes appctx.SprintProcessPort
}

func NewCreateSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository, processes appctx.SprintProcessPort) *CreateSprintHandler {
	return &CreateSprintHandler{products: products, sprints: sprints, processes: processes}
}

func (h *CreateSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input CreateSprintInput) (*model.Sprint, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	exists, err := h.sprints.SprintNameExists(ctx, productID, input.Name, kernel.SprintID{})
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrSprintNameDuplicate
	}
	var process *model.SprintProcess
	if input.ProcessID != nil {
		process, err = h.processes.FindByID(ctx, productID, *input.ProcessID)
		if err != nil {
			return nil, err
		}
	} else {
		process, err = h.processes.GetOrSeedDefault(ctx, productID, actor)
		if err != nil {
			return nil, err
		}
	}
	owner := input.OwnerID
	if owner == nil {
		owner = &actor
	}
	sprint := model.NewSprint(productID, process, input.Name, input.Goal, owner, actor)
	sprint.PlannedStart = input.PlannedStart
	sprint.PlannedEnd = input.PlannedEnd
	if err := h.sprints.SaveSprint(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

type UpdateSprintInput struct {
	Name             *string
	Goal             *string
	PlannedStart     *time.Time
	PlannedEnd       *time.Time
	OwnerID          *kernel.MemberID
	Stages           []model.SprintStage
	StageMigrations  map[string]string
}

type UpdateSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	board    *service.SprintBoardNotifier
}

func NewUpdateSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository, board *service.SprintBoardNotifier) *UpdateSprintHandler {
	return &UpdateSprintHandler{products: products, sprints: sprints, board: board}
}

func (h *UpdateSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID, input UpdateSprintInput) (*model.Sprint, error) {
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name != sprint.Name {
			exists, err := h.sprints.SprintNameExists(ctx, productID, name, sprint.ID)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, domain.ErrSprintNameDuplicate
			}
			sprint.Name = name
		}
	}
	if input.Goal != nil {
		sprint.Goal = strings.TrimSpace(*input.Goal)
	}
	if input.PlannedStart != nil {
		sprint.PlannedStart = input.PlannedStart
	}
	if input.PlannedEnd != nil {
		sprint.PlannedEnd = input.PlannedEnd
	}
	if input.OwnerID != nil {
		sprint.OwnerID = input.OwnerID
	}
	if len(input.Stages) > 0 && sprint.Status != model.SprintStatusPlanning {
		if sprint.Status != model.SprintStatusActive {
			return nil, domain.ErrInvalidSprintTransition
		}
		stages, ok := model.ApplyStageWorkflowBindings(sprint.Stages, input.Stages)
		if !ok {
			return nil, domain.ErrInvalidSprintTransition
		}
		sprint.Stages = stages
	} else if len(input.Stages) > 0 {
		items, err := h.sprints.ListWorkItemsBySprint(ctx, sprint.ID)
		if err != nil {
			return nil, err
		}
		stages, moves, err := model.PlanSprintStageUpdate(sprint.Stages, input.Stages, items, input.StageMigrations)
		if err != nil {
			return nil, err
		}
		for _, move := range moves {
			item, err := h.sprints.FindWorkItemByID(ctx, move.WorkItemID)
			if err != nil {
				return nil, err
			}
			item.StageID = move.NewStageID
			item.UpdatedAt = time.Now().UTC()
			if err := h.sprints.UpdateWorkItem(ctx, item); err != nil {
				return nil, err
			}
		}
		sprint.Stages = stages
	}
	sprint.UpdatedAt = time.Now().UTC()
	if err := h.sprints.UpdateSprint(ctx, sprint); err != nil {
		return nil, err
	}
	if h.board != nil && len(input.Stages) > 0 {
		h.board.BoardChanged(sprint.ID, "stages_updated")
	}
	return sprint, nil
}

type StartSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewStartSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository) *StartSprintHandler {
	return &StartSprintHandler{products: products, sprints: sprints}
}

func (h *StartSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (*model.Sprint, error) {
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	if err := sprint.Start(time.Now().UTC()); err != nil {
		return nil, domain.ErrInvalidSprintTransition
	}
	if err := h.sprints.UpdateSprint(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

type CompleteSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	plans    repository.ReleasePlanRepository
}

func NewCompleteSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository, plans repository.ReleasePlanRepository) *CompleteSprintHandler {
	return &CompleteSprintHandler{products: products, sprints: sprints, plans: plans}
}

func (h *CompleteSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (*model.Sprint, error) {
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	blocking, err := h.plans.HasBlockingPlansBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if blocking {
		return nil, domain.ErrSprintBlockingRelease
	}
	if err := sprint.Complete(time.Now().UTC()); err != nil {
		return nil, domain.ErrInvalidSprintTransition
	}
	if err := h.sprints.UpdateSprint(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

type ArchiveSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewArchiveSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository) *ArchiveSprintHandler {
	return &ArchiveSprintHandler{products: products, sprints: sprints}
}

func (h *ArchiveSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (*model.Sprint, error) {
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	if err := sprint.Archive(time.Now().UTC()); err != nil {
		return nil, domain.ErrInvalidSprintTransition
	}
	if err := h.sprints.UpdateSprint(ctx, sprint); err != nil {
		return nil, err
	}
	return sprint, nil
}

type CreateWorkItemInput struct {
	StageID     string
	Type        model.WorkItemType
	Title       string
	Description string
	Priority    model.WorkItemPriority
	AssigneeID  *kernel.MemberID
	ProjectIDs  []string
	Labels      []string
	SortOrder   int
}

type CreateWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	board    *service.SprintBoardNotifier
}

func NewCreateWorkItemHandler(products repository.ProductRepository, sprints repository.SprintRepository, board *service.SprintBoardNotifier) *CreateWorkItemHandler {
	return &CreateWorkItemHandler{products: products, sprints: sprints, board: board}
}

func (h *CreateWorkItemHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID, actor kernel.MemberID, input CreateWorkItemInput) (*model.WorkItem, error) {
	sprint, err := loadSprint(ctx, h.products, h.sprints, productID, sprintID)
	if err != nil {
		return nil, err
	}
	stageID := input.StageID
	if stageID == "" {
		stageID = sprint.FirstStageID()
	}
	if !sprint.HasStage(stageID) {
		return nil, domain.ErrInvalidStage
	}
	item := model.NewWorkItem(sprintID, stageID, input.Title, input.Type, input.Priority, actor)
	item.Description = strings.TrimSpace(input.Description)
	item.AssigneeID = input.AssigneeID
	item.ProjectIDs = append([]string(nil), input.ProjectIDs...)
	item.Labels = append([]string(nil), input.Labels...)
	item.SortOrder = input.SortOrder
	if err := h.sprints.SaveWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if h.board != nil {
		h.board.BoardChanged(sprintID, "created")
	}
	return item, nil
}

type UpdateWorkItemInput struct {
	Title         *string
	Description   *string
	Type          *model.WorkItemType
	Priority      *model.WorkItemPriority
	Status        *model.WorkItemStatus
	AssigneeID    *kernel.MemberID
	ClearAssignee bool
	ProjectIDs    []string
	Labels        []string
	BlockedReason *string
}

type UpdateWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewUpdateWorkItemHandler(products repository.ProductRepository, sprints repository.SprintRepository) *UpdateWorkItemHandler {
	return &UpdateWorkItemHandler{products: products, sprints: sprints}
}

func (h *UpdateWorkItemHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID, input UpdateWorkItemInput) (*model.WorkItem, error) {
	item, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return nil, err
	}
	if sprint.Status == model.SprintStatusArchived {
		return nil, domain.ErrSprintNotActive
	}
	if input.Title != nil {
		item.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		item.Description = strings.TrimSpace(*input.Description)
	}
	if input.Type != nil {
		item.Type = *input.Type
	}
	if input.Priority != nil {
		item.Priority = *input.Priority
	}
	if input.Status != nil {
		item.Status = *input.Status
	}
	if input.ClearAssignee {
		item.AssigneeID = nil
	} else if input.AssigneeID != nil {
		item.AssigneeID = input.AssigneeID
	}
	if input.ProjectIDs != nil {
		item.ProjectIDs = append([]string(nil), input.ProjectIDs...)
	}
	if input.Labels != nil {
		item.Labels = append([]string(nil), input.Labels...)
	}
	if input.BlockedReason != nil {
		item.BlockedReason = strings.TrimSpace(*input.BlockedReason)
	}
	item.UpdatedAt = time.Now().UTC()
	if err := h.sprints.UpdateWorkItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

type MoveWorkItemInput struct {
	StageID   string
	SortOrder int
	Version   int
	Actor     kernel.MemberID
}

type MoveWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	executor *service.SprintWorkItemExecutor
	board    *service.SprintBoardNotifier
}

func NewMoveWorkItemHandler(
	products repository.ProductRepository,
	sprints repository.SprintRepository,
	executor *service.SprintWorkItemExecutor,
	board *service.SprintBoardNotifier,
) *MoveWorkItemHandler {
	return &MoveWorkItemHandler{products: products, sprints: sprints, executor: executor, board: board}
}

func (h *MoveWorkItemHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID, input MoveWorkItemInput) (*model.WorkItem, error) {
	item, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != model.SprintStatusActive {
		return nil, domain.ErrSprintNotActive
	}
	if item.Version != input.Version {
		return nil, domain.ErrWorkItemVersionConflict
	}
	if !sprint.HasStage(input.StageID) {
		return nil, domain.ErrInvalidStage
	}
	entered := item.StageID != input.StageID
	if entered {
		allItems, err := h.sprints.ListWorkItemsBySprint(ctx, sprint.ID)
		if err != nil {
			return nil, err
		}
		execs, err := h.sprints.ListExecutionsByWorkItem(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if err := model.CheckStageExit(sprint, item, input.StageID, execs); err != nil {
			return nil, err
		}
		if err := model.CheckStageEntry(sprint, item, input.StageID, allItems, execs); err != nil {
			return nil, err
		}
	}
	item.StageID = input.StageID
	item.SortOrder = input.SortOrder
	item.UpdatedAt = time.Now().UTC()
	if err := h.sprints.UpdateWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if entered {
		h.triggerAutoWorkflows(ctx, sprint, item, input.Actor)
	}
	if h.board != nil {
		h.board.BoardChanged(sprint.ID, "moved")
	}
	return item, nil
}

// triggerAutoWorkflows starts the stage's auto bindings. A failed start must not undo the move,
// so it is recorded on the card instead of being returned.
func (h *MoveWorkItemHandler) triggerAutoWorkflows(ctx context.Context, sprint *model.Sprint, item *model.WorkItem, actor kernel.MemberID) {
	stage := sprint.StageByID(item.StageID)
	if h.executor == nil || stage == nil {
		return
	}
	for _, binding := range stage.Workflows {
		if binding.TriggerMode != model.StageTriggerAuto {
			continue
		}
		runInput := map[string]any{}
		if branch, _ := item.Delivery["branch"].(string); branch != "" {
			runInput["branch"] = branch
		}
		if _, err := h.executor.Execute(ctx, sprint, item, stage.ID, binding.WorkflowID, actor, runInput); err != nil {
			if item.Delivery == nil {
				item.Delivery = map[string]any{}
			}
			item.Delivery["last_workflow_id"] = binding.WorkflowID
			item.Delivery["last_workflow_status"] = "failed"
			item.Delivery["last_workflow_error"] = err.Error()
			_ = h.sprints.UpdateWorkItem(ctx, item)
		}
	}
}

type DeleteWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	board    *service.SprintBoardNotifier
}

func NewDeleteWorkItemHandler(products repository.ProductRepository, sprints repository.SprintRepository, board *service.SprintBoardNotifier) *DeleteWorkItemHandler {
	return &DeleteWorkItemHandler{products: products, sprints: sprints, board: board}
}

func (h *DeleteWorkItemHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID) error {
	_, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return err
	}
	if sprint.Status == model.SprintStatusArchived {
		return domain.ErrSprintNotActive
	}
	if err := h.sprints.DeleteWorkItem(ctx, workItemID); err != nil {
		return err
	}
	if h.board != nil {
		h.board.BoardChanged(sprint.ID, "deleted")
	}
	return nil
}

func loadSprint(ctx context.Context, products repository.ProductRepository, sprints repository.SprintRepository, productID kernel.ProductID, sprintID kernel.SprintID) (*model.Sprint, error) {
	if _, err := products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	sprint, err := sprints.FindSprintByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProductID != productID {
		return nil, domain.ErrSprintNotFound
	}
	return sprint, nil
}

type ExecuteWorkItemInput struct {
	WorkflowID string
	StageID    string
	Input      map[string]any
}

type ExecuteWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	executor *service.SprintWorkItemExecutor
	board    *service.SprintBoardNotifier
}

func NewExecuteWorkItemHandler(
	products repository.ProductRepository,
	sprints repository.SprintRepository,
	executor *service.SprintWorkItemExecutor,
	board *service.SprintBoardNotifier,
) *ExecuteWorkItemHandler {
	return &ExecuteWorkItemHandler{products: products, sprints: sprints, executor: executor, board: board}
}

func (h *ExecuteWorkItemHandler) Handle(
	ctx context.Context,
	productID kernel.ProductID,
	workItemID kernel.WorkItemID,
	actor kernel.MemberID,
	input ExecuteWorkItemInput,
) (*service.ExecuteWorkItemResult, error) {
	item, sprint, err := loadWorkItem(ctx, h.products, h.sprints, productID, workItemID)
	if err != nil {
		return nil, err
	}
	result, err := h.executor.Execute(ctx, sprint, item, input.StageID, input.WorkflowID, actor, input.Input)
	if err != nil {
		return nil, err
	}
	if h.board != nil {
		h.board.BoardChanged(sprint.ID, "executed")
	}
	return result, nil
}

func loadWorkItem(ctx context.Context, products repository.ProductRepository, sprints repository.SprintRepository, productID kernel.ProductID, workItemID kernel.WorkItemID) (*model.WorkItem, *model.Sprint, error) {
	item, err := sprints.FindWorkItemByID(ctx, workItemID)
	if err != nil {
		return nil, nil, err
	}
	sprint, err := loadSprint(ctx, products, sprints, productID, item.SprintID)
	if err != nil {
		return nil, nil, err
	}
	return item, sprint, nil
}
