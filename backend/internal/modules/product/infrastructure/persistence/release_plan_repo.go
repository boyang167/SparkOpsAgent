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
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type releasePlanModel struct {
	ID                  string `gorm:"column:id;primaryKey"`
	ProductID           string
	ProductPlanID       *string
	SprintID            *string
	Name                string
	Description         string `gorm:"type:text"`
	ManagerID           string
	ApproverID          *string
	Status              string
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ScheduleExecuteTime *time.Time
	ApprovalConfig      string `gorm:"type:jsonb"`
	HookSettings        string `gorm:"type:jsonb"`
	HookState           *string `gorm:"type:jsonb"`
	CreatedBy           string
	Version             int
	postgres.AuditColumns
	DeletedAt gorm.DeletedAt
}

func (releasePlanModel) TableName() string { return "product.release_plans" }

type releasePlanJobModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	ReleasePlanID string
	SortOrder     int
	Name          string
	Type          string
	OwnerID       string
	Status        string
	TextContent   string `gorm:"type:text"`
	TextFormat    string
	TextChecked   bool
	WorkflowRef   string `gorm:"type:jsonb"`
	ExecutionRef  string `gorm:"type:jsonb"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (releasePlanJobModel) TableName() string { return "product.release_plan_jobs" }

type releasePlanAuditLogModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	ReleasePlanID string
	ActorID       string
	Action        string
	Detail        string `gorm:"type:jsonb"`
	CreatedAt     time.Time
}

func (releasePlanAuditLogModel) TableName() string { return "product.release_plan_audit_logs" }

type releasePlanVersionModel struct {
	ReleasePlanID   string `gorm:"column:release_plan_id;primaryKey"`
	Version         int    `gorm:"column:version;primaryKey"`
	PreviousVersion int
	ActorID         string
	Verb            string
	SectionKey      string
	SectionName     string
	Snapshot        string `gorm:"type:jsonb"`
	CreatedAt       time.Time
}

func (releasePlanVersionModel) TableName() string { return "product.release_plan_versions" }

type ReleasePlanRepository struct{ db *gorm.DB }

func NewReleasePlanRepository(db *gorm.DB) repository.ReleasePlanRepository {
	return &ReleasePlanRepository{db: db}
}

func releasePlanAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&releasePlanModel{}, &releasePlanJobModel{}, &releasePlanAuditLogModel{}, &releasePlanVersionModel{},
		&releasePlanTemplateModel{},
	)
}

func (r *ReleasePlanRepository) SavePlan(ctx context.Context, plan *model.ReleasePlan) error {
	row, err := toReleasePlanModel(plan)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) UpdatePlan(ctx context.Context, plan *model.ReleasePlan) error {
	row, err := toReleasePlanModel(plan)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name": row.Name, "description": row.Description, "manager_id": row.ManagerID,
		"approver_id": row.ApproverID, "status": row.Status, "sprint_id": row.SprintID,
		"release_window_start": row.ReleaseWindowStart, "release_window_end": row.ReleaseWindowEnd,
		"schedule_execute_time": row.ScheduleExecuteTime, "approval_config": row.ApprovalConfig,
		"hook_settings": row.HookSettings, "hook_state": row.HookState,
		"version": row.Version, "updated_at": time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Model(&releasePlanModel{}).Where("id = ?", row.ID).Updates(updates).Error
}

func (r *ReleasePlanRepository) DeletePlan(ctx context.Context, planID kernel.ReleasePlanID) error {
	pid := planID.String()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("release_plan_id = ?", pid).Delete(&releasePlanJobModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("release_plan_id = ?", pid).Delete(&releasePlanAuditLogModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", pid).Delete(&releasePlanModel{}).Error
	})
}

func (r *ReleasePlanRepository) FindPlanByID(ctx context.Context, id kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	var row releasePlanModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanNotFound
	}
	if err != nil {
		return nil, err
	}
	plan, err := rowToReleasePlan(row)
	if err != nil {
		return nil, err
	}
	jobs, err := r.ListJobsByPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	plan.Jobs = jobs
	return plan, nil
}

func (r *ReleasePlanRepository) ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ReleasePlan, error) {
	var rows []releasePlanModel
	err := r.db.WithContext(ctx).Where("product_id = ?", productID.String()).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToReleasePlans(rows)
}

func (r *ReleasePlanRepository) ListPlansByProductAndSprint(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) ([]*model.ReleasePlan, error) {
	var rows []releasePlanModel
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND sprint_id = ?", productID.String(), sprintID.String()).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToReleasePlans(rows)
}

func blockingReleasePlanStatuses() []string {
	return []string{
		string(model.ReleasePlanStatusExecuting),
		string(model.ReleasePlanStatusWaitForApproval),
	}
}

func (r *ReleasePlanRepository) HasBlockingPlansBySprint(ctx context.Context, sprintID kernel.SprintID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&releasePlanModel{}).
		Where("sprint_id = ? AND status IN ?", sprintID.String(), blockingReleasePlanStatuses()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ReleasePlanRepository) ListBlockingPlansBySprint(ctx context.Context, sprintID kernel.SprintID) ([]*model.ReleasePlan, error) {
	var rows []releasePlanModel
	err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND status IN ?", sprintID.String(), blockingReleasePlanStatuses()).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToReleasePlans(rows)
}

// releasePlanListColumns 列表查询不读取需求说明与 Hook 等大字段。
var releasePlanListColumns = []string{
	"id", "product_id", "product_plan_id", "sprint_id", "name", "manager_id", "approver_id", "status",
	"release_window_start", "release_window_end", "schedule_execute_time", "approval_config",
	"created_by", "version", "created_at", "updated_at",
}

func prefixedColumns(prefix string, cols []string) []string {
	out := make([]string, len(cols))
	for i, col := range cols {
		out[i] = prefix + col
	}
	return out
}

func (r *ReleasePlanRepository) ListPlansByProductFiltered(ctx context.Context, productID kernel.ProductID, sprintID *kernel.SprintID, filter repository.ReleasePlanListFilter) ([]*model.ReleasePlan, error) {
	q := r.db.WithContext(ctx).Select(releasePlanListColumns).Where("product_id = ?", productID.String())
	if sprintID != nil {
		q = q.Where("sprint_id = ?", sprintID.String())
	}
	q = applyReleasePlanListFilter(q, filter, "")
	var rows []releasePlanModel
	if err := q.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToReleasePlans(rows)
}

func (r *ReleasePlanRepository) ListPlansByWorkspace(ctx context.Context, wsID kernel.WorkspaceID, filter repository.ReleasePlanListFilter) ([]*model.ReleasePlan, error) {
	q := r.db.WithContext(ctx).
		Table("product.release_plans AS rp").
		Select(prefixedColumns("rp.", releasePlanListColumns)).
		Joins("INNER JOIN product.products AS p ON p.id = rp.product_id").
		Where("p.workspace_id = ? AND rp.deleted_at IS NULL", wsID.String())
	q = applyReleasePlanListFilter(q, filter, "rp.")
	var rows []releasePlanModel
	if err := q.Order("rp.created_at DESC, rp.id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToReleasePlans(rows)
}

func applyReleasePlanListFilter(q *gorm.DB, filter repository.ReleasePlanListFilter, prefix string) *gorm.DB {
	if len(filter.Statuses) > 0 {
		statuses := make([]string, 0, len(filter.Statuses))
		for _, s := range filter.Statuses {
			statuses = append(statuses, string(s))
		}
		q = q.Where(prefix+"status IN ?", statuses)
	}
	if filter.From != nil {
		q = q.Where(prefix+"release_window_end >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where(prefix+"release_window_start <= ?", *filter.To)
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		q = q.Where(prefix+"name ILIKE ? ESCAPE '\\'", "%"+escapeLikePattern(kw)+"%")
	}
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	return q
}

func escapeLikePattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func rowsToReleasePlans(rows []releasePlanModel) ([]*model.ReleasePlan, error) {
	out := make([]*model.ReleasePlan, 0, len(rows))
	for _, row := range rows {
		plan, err := rowToReleasePlan(row)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func (r *ReleasePlanRepository) SaveJob(ctx context.Context, job *model.ReleasePlanJob) error {
	row, err := toReleasePlanJobModel(job)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) UpdateJob(ctx context.Context, job *model.ReleasePlanJob) error {
	row, err := toReleasePlanJobModel(job)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&releasePlanJobModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"sort_order": row.SortOrder, "name": row.Name, "type": row.Type, "owner_id": row.OwnerID,
		"status": row.Status, "text_content": row.TextContent, "text_format": row.TextFormat, "text_checked": row.TextChecked,
		"workflow_ref": row.WorkflowRef, "execution_ref": row.ExecutionRef, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ReleasePlanRepository) DeleteJob(ctx context.Context, jobID kernel.ReleasePlanJobID) error {
	return r.db.WithContext(ctx).Where("id = ?", jobID.String()).Delete(&releasePlanJobModel{}).Error
}

func (r *ReleasePlanRepository) ListJobsByPlan(ctx context.Context, planID kernel.ReleasePlanID) ([]model.ReleasePlanJob, error) {
	var rows []releasePlanJobModel
	err := r.db.WithContext(ctx).Where("release_plan_id = ?", planID.String()).Order("sort_order ASC, created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.ReleasePlanJob, 0, len(rows))
	for _, row := range rows {
		job, err := rowToReleasePlanJob(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, nil
}

func (r *ReleasePlanRepository) SaveAuditLog(ctx context.Context, log *model.ReleasePlanAuditLog) error {
	detail := string(log.Detail)
	if detail == "" {
		detail = "{}"
	}
	row := releasePlanAuditLogModel{
		ID: uuid.NewString(), ReleasePlanID: log.ReleasePlanID.String(),
		ActorID: log.ActorID.String(), Action: log.Action, Detail: detail,
		CreatedAt: log.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) FindJobByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.ReleasePlanJob, *model.ReleasePlan, error) {
	var row releasePlanJobModel
	err := r.db.WithContext(ctx).
		Where("execution_ref->>'workflow_run_id' = ?", runID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, domain.ErrReleasePlanJobNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	job, err := rowToReleasePlanJob(row)
	if err != nil {
		return nil, nil, err
	}
	plan, err := r.FindPlanByID(ctx, job.ReleasePlanID)
	if err != nil {
		return nil, nil, err
	}
	for i := range plan.Jobs {
		if plan.Jobs[i].ID == job.ID {
			return &plan.Jobs[i], plan, nil
		}
	}
	return nil, nil, domain.ErrReleasePlanJobNotFound
}

func (r *ReleasePlanRepository) FindPlanByHookToken(ctx context.Context, token string) (*model.ReleasePlan, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domain.ErrReleasePlanNotFound
	}
	var row releasePlanModel
	err := r.db.WithContext(ctx).
		Where("hook_state->>'token' = ?", token).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanNotFound
	}
	if err != nil {
		return nil, err
	}
	plan, err := rowToReleasePlan(row)
	if err != nil {
		return nil, err
	}
	jobs, err := r.ListJobsByPlan(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Jobs = jobs
	return plan, nil
}

func (r *ReleasePlanRepository) FindPlanByApprovalToken(ctx context.Context, token string) (*model.ReleasePlan, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domain.ErrReleasePlanNotFound
	}
	var row releasePlanModel
	err := r.db.WithContext(ctx).
		Where("approval_config->'instance'->>'token' = ?", token).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanNotFound
	}
	if err != nil {
		return nil, err
	}
	plan, err := rowToReleasePlan(row)
	if err != nil {
		return nil, err
	}
	jobs, err := r.ListJobsByPlan(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	plan.Jobs = jobs
	return plan, nil
}

func (r *ReleasePlanRepository) ListDueScheduledPlans(ctx context.Context, now time.Time) ([]*model.ReleasePlan, error) {
	var rows []releasePlanModel
	err := r.db.WithContext(ctx).
		Where("status = ? AND schedule_execute_time IS NOT NULL AND schedule_execute_time <= ?",
			string(model.ReleasePlanStatusExecuting), now).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToReleasePlansWithJobs(ctx, r, rows)
}

func rowsToReleasePlansWithJobs(ctx context.Context, r *ReleasePlanRepository, rows []releasePlanModel) ([]*model.ReleasePlan, error) {
	out := make([]*model.ReleasePlan, 0, len(rows))
	for _, row := range rows {
		plan, err := rowToReleasePlan(row)
		if err != nil {
			return nil, err
		}
		jobs, err := r.ListJobsByPlan(ctx, plan.ID)
		if err != nil {
			return nil, err
		}
		plan.Jobs = jobs
		out = append(out, plan)
	}
	return out, nil
}

func (r *ReleasePlanRepository) UpdatePlanVersion(ctx context.Context, planID kernel.ReleasePlanID, version int) error {
	return r.db.WithContext(ctx).Model(&releasePlanModel{}).
		Where("id = ?", planID.String()).
		Updates(map[string]any{"version": version, "updated_at": time.Now().UTC()}).Error
}

func (r *ReleasePlanRepository) SaveVersion(ctx context.Context, version *model.ReleasePlanVersion) error {
	raw, err := json.Marshal(version.Snapshot)
	if err != nil {
		return err
	}
	row := releasePlanVersionModel{
		ReleasePlanID: version.ReleasePlanID.String(), Version: version.Version,
		PreviousVersion: version.PreviousVersion, ActorID: version.ActorID.String(),
		Verb: version.Verb, SectionKey: version.SectionKey, SectionName: version.SectionName,
		Snapshot: string(raw), CreatedAt: version.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) ListVersionsByPlan(ctx context.Context, planID kernel.ReleasePlanID, limit int) ([]model.ReleasePlanVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []releasePlanVersionModel
	err := r.db.WithContext(ctx).
		Where("release_plan_id = ?", planID.String()).
		Order("version DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToReleasePlanVersions(rows)
}

func (r *ReleasePlanRepository) ListPlanIDsWithoutVersions(ctx context.Context, limit int) ([]kernel.ReleasePlanID, error) {
	if limit <= 0 {
		limit = 200
	}
	var ids []string
	err := r.db.WithContext(ctx).
		Table("product.release_plans AS rp").
		Select("rp.id").
		Where("rp.deleted_at IS NULL").
		Where("NOT EXISTS (SELECT 1 FROM product.release_plan_versions v WHERE v.release_plan_id = rp.id)").
		Order("rp.created_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	out := make([]kernel.ReleasePlanID, 0, len(ids))
	for _, id := range ids {
		planID, err := kernel.ParseReleasePlanID(id)
		if err != nil {
			continue
		}
		out = append(out, planID)
	}
	return out, nil
}

func (r *ReleasePlanRepository) GetVersion(ctx context.Context, planID kernel.ReleasePlanID, version int) (*model.ReleasePlanVersion, error) {
	var row releasePlanVersionModel
	err := r.db.WithContext(ctx).
		Where("release_plan_id = ? AND version = ?", planID.String(), version).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	out, err := rowToReleasePlanVersion(row)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ReleasePlanRepository) ListAuditLogs(ctx context.Context, planID kernel.ReleasePlanID, limit int) ([]model.ReleasePlanAuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []releasePlanAuditLogModel
	err := r.db.WithContext(ctx).Where("release_plan_id = ?", planID.String()).Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.ReleasePlanAuditLog, 0, len(rows))
	for _, row := range rows {
		actorID, _ := kernel.ParseMemberID(row.ActorID)
		planIDParsed, _ := kernel.ParseReleasePlanID(row.ReleasePlanID)
		out = append(out, model.ReleasePlanAuditLog{
			ID: row.ID, ReleasePlanID: planIDParsed, ActorID: actorID,
			Action: row.Action, Detail: json.RawMessage(row.Detail), CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func toReleasePlanModel(plan *model.ReleasePlan) (releasePlanModel, error) {
	approval, err := json.Marshal(plan.ApprovalConfig)
	if err != nil {
		return releasePlanModel{}, err
	}
	hookSettings, err := json.Marshal(plan.HookSettings)
	if err != nil {
		return releasePlanModel{}, err
	}
	row := releasePlanModel{
		ID: plan.ID.String(), ProductID: plan.ProductID.String(), Name: plan.Name,
		Description: plan.Description, ManagerID: plan.ManagerID.String(), Status: string(plan.Status),
		ReleaseWindowStart: plan.ReleaseWindowStart, ReleaseWindowEnd: plan.ReleaseWindowEnd,
		ScheduleExecuteTime: plan.ScheduleExecuteTime, ApprovalConfig: string(approval),
		HookSettings: string(hookSettings),
		CreatedBy: plan.CreatedBy.String(), Version: plan.Version,
		AuditColumns: postgres.AuditColumns{CreatedAt: plan.CreatedAt, UpdatedAt: plan.UpdatedAt},
	}
	if plan.HookState != nil {
		raw, err := json.Marshal(plan.HookState)
		if err != nil {
			return releasePlanModel{}, err
		}
		s := string(raw)
		row.HookState = &s
	}
	if row.Version <= 0 {
		row.Version = 1
	}
	if plan.ProductPlanID != nil {
		v := plan.ProductPlanID.String()
		row.ProductPlanID = &v
	}
	if plan.SprintID != nil {
		v := plan.SprintID.String()
		row.SprintID = &v
	}
	if plan.ApproverID != nil {
		v := plan.ApproverID.String()
		row.ApproverID = &v
	}
	return row, nil
}

func rowToReleasePlan(row releasePlanModel) (*model.ReleasePlan, error) {
	id, err := kernel.ParseReleasePlanID(row.ID)
	if err != nil {
		return nil, err
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, err
	}
	managerID, err := kernel.ParseMemberID(row.ManagerID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var approval model.ApprovalConfig
	_ = json.Unmarshal([]byte(row.ApprovalConfig), &approval)
	var hookSettings model.ReleasePlanHookSettings
	if row.HookSettings != "" {
		_ = json.Unmarshal([]byte(row.HookSettings), &hookSettings)
	}
	plan := &model.ReleasePlan{
		ID: id, ProductID: productID, Name: row.Name, Description: row.Description,
		ManagerID: managerID, Status: model.ReleasePlanStatus(row.Status),
		ReleaseWindowStart: row.ReleaseWindowStart, ReleaseWindowEnd: row.ReleaseWindowEnd,
		ScheduleExecuteTime: row.ScheduleExecuteTime, ApprovalConfig: approval,
		HookSettings: hookSettings,
		CreatedBy: createdBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Version: row.Version,
	}
	if row.HookState != nil && *row.HookState != "" {
		var hookState model.ReleasePlanHookState
		if err := json.Unmarshal([]byte(*row.HookState), &hookState); err == nil {
			plan.HookState = &hookState
		}
	}
	if plan.Version <= 0 {
		plan.Version = 1
	}
	if row.ProductPlanID != nil {
		pp, err := kernel.ParseProductPlanID(*row.ProductPlanID)
		if err == nil {
			plan.ProductPlanID = &pp
		}
	}
	if row.SprintID != nil {
		sid, err := kernel.ParseSprintID(*row.SprintID)
		if err == nil {
			plan.SprintID = &sid
		}
	}
	if row.ApproverID != nil {
		approver, err := kernel.ParseMemberID(*row.ApproverID)
		if err == nil {
			plan.ApproverID = &approver
		}
	}
	return plan, nil
}

func toReleasePlanJobModel(job *model.ReleasePlanJob) (releasePlanJobModel, error) {
	wf := string(job.WorkflowRef)
	if wf == "" {
		wf = "null"
	}
	ex := string(job.ExecutionRef)
	if ex == "" {
		ex = "null"
	}
	return releasePlanJobModel{
		ID: job.ID.String(), ReleasePlanID: job.ReleasePlanID.String(), SortOrder: job.SortOrder,
		Name: job.Name, Type: string(job.Type), OwnerID: job.OwnerID.String(), Status: string(job.Status),
		TextContent: job.TextContent, TextFormat: job.TextFormat, TextChecked: job.TextChecked,
		WorkflowRef: wf, ExecutionRef: ex, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}, nil
}

func rowToReleasePlanJob(row releasePlanJobModel) (*model.ReleasePlanJob, error) {
	id, err := kernel.ParseReleasePlanJobID(row.ID)
	if err != nil {
		return nil, err
	}
	planID, err := kernel.ParseReleasePlanID(row.ReleasePlanID)
	if err != nil {
		return nil, err
	}
	ownerID, err := kernel.ParseMemberID(row.OwnerID)
	if err != nil {
		return nil, err
	}
	return &model.ReleasePlanJob{
		ID: id, ReleasePlanID: planID, SortOrder: row.SortOrder, Name: row.Name,
		Type: model.ReleasePlanJobType(row.Type), OwnerID: ownerID, Status: model.ReleasePlanJobStatus(row.Status),
		TextContent: row.TextContent, TextFormat: row.TextFormat, TextChecked: row.TextChecked,
		WorkflowRef: json.RawMessage(row.WorkflowRef), ExecutionRef: json.RawMessage(row.ExecutionRef),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func rowsToReleasePlanVersions(rows []releasePlanVersionModel) ([]model.ReleasePlanVersion, error) {
	out := make([]model.ReleasePlanVersion, 0, len(rows))
	for _, row := range rows {
		item, err := rowToReleasePlanVersion(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func rowToReleasePlanVersion(row releasePlanVersionModel) (model.ReleasePlanVersion, error) {
	planID, err := kernel.ParseReleasePlanID(row.ReleasePlanID)
	if err != nil {
		return model.ReleasePlanVersion{}, err
	}
	actorID, err := kernel.ParseMemberID(row.ActorID)
	if err != nil {
		return model.ReleasePlanVersion{}, err
	}
	var snapshot model.ReleasePlanVersionSnapshot
	if err := json.Unmarshal([]byte(row.Snapshot), &snapshot); err != nil {
		return model.ReleasePlanVersion{}, err
	}
	return model.ReleasePlanVersion{
		ReleasePlanID: planID, Version: row.Version, PreviousVersion: row.PreviousVersion,
		ActorID: actorID, Verb: row.Verb, SectionKey: row.SectionKey, SectionName: row.SectionName,
		Snapshot: snapshot, CreatedAt: row.CreatedAt,
	}, nil
}
