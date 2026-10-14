package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type sprintModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	ProductID      string
	ProcessID      string
	ProcessVersion int
	Stages         string `gorm:"type:jsonb"`
	Name           string
	Goal           string `gorm:"type:text"`
	Status         string
	PlannedStart   *time.Time
	PlannedEnd     *time.Time
	ActualStart    *time.Time
	ActualEnd      *time.Time
	OwnerID        *string
	ExternalRef    string `gorm:"type:jsonb"`
	LegacyPlanID   *string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt
}

func (sprintModel) TableName() string { return "product.sprints" }

type workItemModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	SprintID      string
	StageID       string
	ParentID      *string
	SortOrder     int
	Type          string
	Title         string
	Description   string `gorm:"type:text"`
	Priority      string
	Status        string
	AssigneeID    *string
	ReporterID    *string
	ProjectIDs    string `gorm:"type:jsonb"`
	Labels        string `gorm:"type:jsonb"`
	ExternalRef   string `gorm:"type:jsonb"`
	Delivery      string `gorm:"type:jsonb"`
	BlockedReason string `gorm:"type:text"`
	Version       int
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt
}

func (workItemModel) TableName() string { return "product.work_items" }

type workItemExecutionModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	WorkItemID    string
	StageID       string
	WorkflowID    string
	WorkflowRunID string
	TriggeredBy   string
	Status        string
	Artifacts     string `gorm:"type:jsonb"`
	ErrorMessage  string `gorm:"type:text"`
	StartedAt     time.Time
	FinishedAt    *time.Time
}

func (workItemExecutionModel) TableName() string { return "product.work_item_executions" }

type SprintRepository struct{ db *gorm.DB }

func NewSprintRepository(db *gorm.DB) repository.SprintRepository {
	return &SprintRepository{db: db}
}

func sprintAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&sprintModel{},
		&workItemModel{},
		&workItemExecutionModel{},
	)
}

func (r *SprintRepository) SaveSprint(ctx context.Context, sprint *model.Sprint) error {
	row, err := toSprintModel(sprint)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *SprintRepository) UpdateSprint(ctx context.Context, sprint *model.Sprint) error {
	row, err := toSprintModel(sprint)
	if err != nil {
		return err
	}
	extRef := row.ExternalRef
	if extRef == "" {
		extRef = "{}"
	}
	updates := map[string]any{
		"name": row.Name, "goal": row.Goal, "status": row.Status,
		"planned_start": row.PlannedStart, "planned_end": row.PlannedEnd,
		"actual_start": row.ActualStart, "actual_end": row.ActualEnd,
		"owner_id": row.OwnerID, "external_ref": extRef, "updated_at": time.Now().UTC(),
	}
	if row.Stages != "" && row.Stages != "[]" {
		updates["stages"] = row.Stages
	}
	return r.db.WithContext(ctx).Model(&sprintModel{}).Where("id = ?", row.ID).Updates(updates).Error
}

func (r *SprintRepository) FindSprintByID(ctx context.Context, id kernel.SprintID) (*model.Sprint, error) {
	var row sprintModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToSprint(row)
}

func (r *SprintRepository) ListActiveSprintsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Sprint, error) {
	var rows []sprintModel
	err := r.db.WithContext(ctx).
		Table("product.sprints AS s").
		Select("s.*").
		Joins("INNER JOIN product.products AS p ON p.id = s.product_id").
		Where("p.workspace_id = ? AND s.status = ? AND s.deleted_at IS NULL", wsID.String(), model.SprintStatusActive).
		Order("s.updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Sprint, 0, len(rows))
	for _, row := range rows {
		sprint, err := rowToSprint(row)
		if err != nil {
			return nil, err
		}
		out = append(out, sprint)
	}
	return out, nil
}

const sprintListColumns = "id, product_id, process_id, process_version, name, goal, status, planned_start, planned_end, actual_start, actual_end, owner_id, created_by, created_at, updated_at"

const workItemCardColumns = "id, sprint_id, stage_id, parent_id, sort_order, type, title, priority, status, assignee_id, project_ids, labels, external_ref, delivery, blocked_reason, version, created_by, created_at, updated_at"

func (r *SprintRepository) ListSprintsByProduct(ctx context.Context, productID kernel.ProductID, status string) ([]*model.Sprint, error) {
	q := r.db.WithContext(ctx).Select(sprintListColumns).Where("product_id = ?", productID.String()).Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []sprintModel
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Sprint, 0, len(rows))
	for _, row := range rows {
		s, err := rowToSprint(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *SprintRepository) SprintNameExists(ctx context.Context, productID kernel.ProductID, name string, excludeID kernel.SprintID) (bool, error) {
	q := r.db.WithContext(ctx).Model(&sprintModel{}).
		Where("product_id = ? AND name = ?", productID.String(), strings.TrimSpace(name))
	if excludeID.UUID != uuid.Nil {
		q = q.Where("id <> ?", excludeID.String())
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SprintRepository) DeleteSprint(ctx context.Context, id kernel.SprintID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sprint_id = ?", id.String()).Delete(&workItemModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&sprintModel{}, "id = ?", id.String()).Error
	})
}

func (r *SprintRepository) SaveWorkItem(ctx context.Context, item *model.WorkItem) error {
	row, err := toWorkItemModel(item)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *SprintRepository) UpdateWorkItem(ctx context.Context, item *model.WorkItem) error {
	row, err := toWorkItemModel(item)
	if err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Model(&workItemModel{}).
		Where("id = ? AND version = ?", row.ID, item.Version).
		Updates(map[string]any{
			"stage_id": row.StageID, "sort_order": row.SortOrder, "type": row.Type,
			"title": row.Title, "description": row.Description, "priority": row.Priority,
			"status": row.Status, "assignee_id": row.AssigneeID, "reporter_id": row.ReporterID,
			"project_ids": row.ProjectIDs, "labels": row.Labels, "external_ref": row.ExternalRef,
			"delivery": row.Delivery, "blocked_reason": row.BlockedReason,
			"version": item.Version + 1, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrWorkItemVersionConflict
	}
	item.Version++
	item.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *SprintRepository) DeleteWorkItem(ctx context.Context, id kernel.WorkItemID) error {
	return r.db.WithContext(ctx).Delete(&workItemModel{}, "id = ?", id.String()).Error
}

func (r *SprintRepository) FindWorkItemByID(ctx context.Context, id kernel.WorkItemID) (*model.WorkItem, error) {
	var row workItemModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToWorkItem(row)
}

func (r *SprintRepository) ListWorkItemsBySprint(ctx context.Context, sprintID kernel.SprintID) ([]*model.WorkItem, error) {
	var rows []workItemModel
	if err := r.db.WithContext(ctx).
		Where("sprint_id = ?", sprintID.String()).
		Order("stage_id ASC, sort_order ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.WorkItem, 0, len(rows))
	for _, row := range rows {
		item, err := rowToWorkItem(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

type sprintWorkItemCountRow struct {
	SprintID string
	Total    int
	Done     int
}

func (r *SprintRepository) CountWorkItemsByProduct(ctx context.Context, productID kernel.ProductID) (map[string][2]int, error) {
	var rows []sprintWorkItemCountRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT w.sprint_id AS sprint_id,
		       COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE w.status = ?) AS done
		FROM product.work_items AS w
		INNER JOIN product.sprints AS s ON s.id = w.sprint_id AND s.deleted_at IS NULL
		WHERE s.product_id = ? AND w.deleted_at IS NULL
		GROUP BY w.sprint_id
	`, model.WorkItemStatusDone, productID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string][2]int, len(rows))
	for _, row := range rows {
		out[row.SprintID] = [2]int{row.Total, row.Done}
	}
	return out, nil
}

func (r *SprintRepository) ListWorkItemCardsBySprint(ctx context.Context, sprintID kernel.SprintID) ([]*model.WorkItem, error) {
	var rows []workItemModel
	if err := r.db.WithContext(ctx).
		Select(workItemCardColumns).
		Where("sprint_id = ?", sprintID.String()).
		Order("stage_id ASC, sort_order ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.WorkItem, 0, len(rows))
	for _, row := range rows {
		item, err := rowToWorkItem(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *SprintRepository) CountWorkItemsBySprint(ctx context.Context, sprintID kernel.SprintID) (total, done int, err error) {
	var totalCount int64
	if err = r.db.WithContext(ctx).Model(&workItemModel{}).Where("sprint_id = ?", sprintID.String()).Count(&totalCount).Error; err != nil {
		return 0, 0, err
	}
	var doneCount int64
	if err = r.db.WithContext(ctx).Model(&workItemModel{}).
		Where("sprint_id = ? AND status = ?", sprintID.String(), model.WorkItemStatusDone).
		Count(&doneCount).Error; err != nil {
		return 0, 0, err
	}
	return int(totalCount), int(doneCount), nil
}

func toSprintModel(s *model.Sprint) (sprintModel, error) {
	stages, err := json.Marshal(s.Stages)
	if err != nil {
		return sprintModel{}, err
	}
	extRef := "{}"
	if len(s.ExternalRef) > 0 {
		b, err := json.Marshal(s.ExternalRef)
		if err != nil {
			return sprintModel{}, err
		}
		extRef = string(b)
	}
	row := sprintModel{
		ID: s.ID.String(), ProductID: s.ProductID.String(), ProcessID: s.ProcessID.String(),
		ProcessVersion: s.ProcessVersion, Stages: string(stages), Name: s.Name, Goal: s.Goal,
		Status: string(s.Status), PlannedStart: s.PlannedStart, PlannedEnd: s.PlannedEnd,
		ActualStart: s.ActualStart, ActualEnd: s.ActualEnd, ExternalRef: extRef,
		CreatedBy: s.CreatedBy.String(), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if s.OwnerID != nil {
		v := s.OwnerID.String()
		row.OwnerID = &v
	}
	if s.LegacyPlanID != nil {
		v := s.LegacyPlanID.String()
		row.LegacyPlanID = &v
	}
	return row, nil
}

func rowToSprint(row sprintModel) (*model.Sprint, error) {
	id, _ := kernel.ParseSprintID(row.ID)
	pid, _ := kernel.ParseProductID(row.ProductID)
	procID, _ := kernel.ParseSprintProcessID(row.ProcessID)
	cby, _ := kernel.ParseMemberID(row.CreatedBy)
	var stages []model.SprintStage
	if row.Stages != "" {
		_ = json.Unmarshal([]byte(row.Stages), &stages)
	}
	s := &model.Sprint{
		ID: id, ProductID: pid, ProcessID: procID, ProcessVersion: row.ProcessVersion,
		Stages: stages, Name: row.Name, Goal: row.Goal, Status: model.SprintStatus(row.Status),
		PlannedStart: row.PlannedStart, PlannedEnd: row.PlannedEnd,
		ActualStart: row.ActualStart, ActualEnd: row.ActualEnd,
		CreatedBy: cby, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.OwnerID != nil {
		oid, err := kernel.ParseMemberID(*row.OwnerID)
		if err == nil {
			s.OwnerID = &oid
		}
	}
	if row.ExternalRef != "" && row.ExternalRef != "{}" {
		_ = json.Unmarshal([]byte(row.ExternalRef), &s.ExternalRef)
	}
	if row.LegacyPlanID != nil {
		lid, err := kernel.ParseProductPlanID(*row.LegacyPlanID)
		if err == nil {
			s.LegacyPlanID = &lid
		}
	}
	return s, nil
}

func (r *SprintRepository) FindSprintByLegacyPlanID(ctx context.Context, planID kernel.ProductPlanID) (*model.Sprint, error) {
	var row sprintModel
	err := r.db.WithContext(ctx).Where("legacy_plan_id = ?", planID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToSprint(row)
}

func toWorkItemModel(item *model.WorkItem) (workItemModel, error) {
	projectIDs, _ := json.Marshal(item.ProjectIDs)
	labels, _ := json.Marshal(item.Labels)
	extRef := "{}"
	if len(item.ExternalRef) > 0 {
		b, _ := json.Marshal(item.ExternalRef)
		extRef = string(b)
	}
	delivery := "{}"
	if len(item.Delivery) > 0 {
		b, _ := json.Marshal(item.Delivery)
		delivery = string(b)
	}
	row := workItemModel{
		ID: item.ID.String(), SprintID: item.SprintID.String(), StageID: item.StageID,
		SortOrder: item.SortOrder, Type: string(item.Type), Title: item.Title,
		Description: item.Description, Priority: string(item.Priority), Status: string(item.Status),
		ProjectIDs: string(projectIDs), Labels: string(labels), ExternalRef: extRef,
		Delivery: delivery, BlockedReason: item.BlockedReason, Version: item.Version,
		CreatedBy: item.CreatedBy.String(), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if item.ParentID != nil {
		v := item.ParentID.String()
		row.ParentID = &v
	}
	if item.AssigneeID != nil {
		v := item.AssigneeID.String()
		row.AssigneeID = &v
	}
	if item.ReporterID != nil {
		v := item.ReporterID.String()
		row.ReporterID = &v
	}
	return row, nil
}

func rowToWorkItem(row workItemModel) (*model.WorkItem, error) {
	id, _ := kernel.ParseWorkItemID(row.ID)
	sid, _ := kernel.ParseSprintID(row.SprintID)
	cby, _ := kernel.ParseMemberID(row.CreatedBy)
	item := &model.WorkItem{
		ID: id, SprintID: sid, StageID: row.StageID, SortOrder: row.SortOrder,
		Type: model.WorkItemType(row.Type), Title: row.Title, Description: row.Description,
		Priority: model.WorkItemPriority(row.Priority), Status: model.WorkItemStatus(row.Status),
		BlockedReason: row.BlockedReason, Version: row.Version,
		CreatedBy: cby, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.ParentID != nil {
		pid, err := kernel.ParseWorkItemID(*row.ParentID)
		if err == nil {
			item.ParentID = &pid
		}
	}
	if row.AssigneeID != nil {
		aid, err := kernel.ParseMemberID(*row.AssigneeID)
		if err == nil {
			item.AssigneeID = &aid
		}
	}
	if row.ReporterID != nil {
		rid, err := kernel.ParseMemberID(*row.ReporterID)
		if err == nil {
			item.ReporterID = &rid
		}
	}
	if row.ProjectIDs != "" {
		_ = json.Unmarshal([]byte(row.ProjectIDs), &item.ProjectIDs)
	}
	if row.Labels != "" {
		_ = json.Unmarshal([]byte(row.Labels), &item.Labels)
	}
	if row.ExternalRef != "" && row.ExternalRef != "{}" {
		_ = json.Unmarshal([]byte(row.ExternalRef), &item.ExternalRef)
	}
	if row.Delivery != "" && row.Delivery != "{}" {
		_ = json.Unmarshal([]byte(row.Delivery), &item.Delivery)
	}
	return item, nil
}

func (r *SprintRepository) SaveExecution(ctx context.Context, exec *model.WorkItemExecution) error {
	row, err := toWorkItemExecutionModel(exec)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *SprintRepository) UpdateExecution(ctx context.Context, exec *model.WorkItemExecution) error {
	row, err := toWorkItemExecutionModel(exec)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&workItemExecutionModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status": row.Status, "artifacts": row.Artifacts, "error_message": row.ErrorMessage,
		"finished_at": row.FinishedAt,
	}).Error
}

func (r *SprintRepository) FindExecutionByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.WorkItemExecution, error) {
	var row workItemExecutionModel
	err := r.db.WithContext(ctx).Where("workflow_run_id = ?", runID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkItemExecutionNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToWorkItemExecution(row)
}

func (r *SprintRepository) ListExecutionsByWorkItem(ctx context.Context, workItemID kernel.WorkItemID) ([]*model.WorkItemExecution, error) {
	var rows []workItemExecutionModel
	if err := r.db.WithContext(ctx).
		Where("work_item_id = ?", workItemID.String()).
		Order("started_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.WorkItemExecution, 0, len(rows))
	for _, row := range rows {
		exec, err := rowToWorkItemExecution(row)
		if err != nil {
			return nil, err
		}
		out = append(out, exec)
	}
	return out, nil
}

func toWorkItemExecutionModel(exec *model.WorkItemExecution) (workItemExecutionModel, error) {
	artifacts := "{}"
	if len(exec.Artifacts) > 0 {
		b, err := json.Marshal(exec.Artifacts)
		if err != nil {
			return workItemExecutionModel{}, err
		}
		artifacts = string(b)
	}
	return workItemExecutionModel{
		ID: exec.ID.String(), WorkItemID: exec.WorkItemID.String(), StageID: exec.StageID,
		WorkflowID: exec.WorkflowID.String(), WorkflowRunID: exec.WorkflowRunID.String(),
		TriggeredBy: exec.TriggeredBy.String(), Status: string(exec.Status),
		Artifacts: artifacts, ErrorMessage: exec.ErrorMessage,
		StartedAt: exec.StartedAt, FinishedAt: exec.FinishedAt,
	}, nil
}

func rowToWorkItemExecution(row workItemExecutionModel) (*model.WorkItemExecution, error) {
	id, _ := kernel.ParseWorkItemExecutionID(row.ID)
	wid, _ := kernel.ParseWorkItemID(row.WorkItemID)
	wfid, _ := kernel.ParseProductWorkflowID(row.WorkflowID)
	runID, _ := kernel.ParseProductWorkflowRunID(row.WorkflowRunID)
	tby, _ := kernel.ParseMemberID(row.TriggeredBy)
	exec := &model.WorkItemExecution{
		ID: id, WorkItemID: wid, StageID: row.StageID, WorkflowID: wfid,
		WorkflowRunID: runID, TriggeredBy: tby, Status: model.WorkItemExecutionStatus(row.Status),
		ErrorMessage: row.ErrorMessage, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
	}
	if row.Artifacts != "" && row.Artifacts != "{}" {
		_ = json.Unmarshal([]byte(row.Artifacts), &exec.Artifacts)
	}
	return exec, nil
}
