package model

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRefSpec struct {
	Mode             string            `json:"mode"`
	WorkflowID       string            `json:"workflow_id"`
	Parameters       map[string]string `json:"parameters,omitempty"`
	WorkflowSnapshot json.RawMessage   `json:"workflow_snapshot,omitempty"`
}

type ExecutionRef struct {
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	BuildID       string `json:"build_id,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	DoneRemark    string `json:"done_remark,omitempty"`
}

type ApprovalProvider string

const (
	ApprovalProviderInternal ApprovalProvider = "internal"
	ApprovalProviderFeishu   ApprovalProvider = "feishu"
	ApprovalProviderDingTalk ApprovalProvider = "dingtalk"
)

type ApprovalInstance struct {
	Token     string     `json:"token,omitempty"`
	Status    string     `json:"status,omitempty"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	ActedAt   *time.Time `json:"acted_at,omitempty"`
	ActorNote string     `json:"actor_note,omitempty"`
}

type ApprovalConfig struct {
	Enabled     bool               `json:"enabled"`
	Provider    ApprovalProvider   `json:"provider,omitempty"`
	WebhookURL  string             `json:"webhook_url,omitempty"`
	ApproverIDs []string           `json:"approver_ids,omitempty"`
	Instance    *ApprovalInstance  `json:"instance,omitempty"`
}

func (c ApprovalConfig) ProviderOrDefault() ApprovalProvider {
	if c.Provider == "" {
		return ApprovalProviderInternal
	}
	return c.Provider
}

func (c ApprovalConfig) UsesExternalApproval() bool {
	if !c.Enabled {
		return false
	}
	switch c.ProviderOrDefault() {
	case ApprovalProviderFeishu, ApprovalProviderDingTalk:
		return true
	default:
		return false
	}
}

type ReleasePlanStatus string

const (
	ReleasePlanStatusPlanning         ReleasePlanStatus = "planning"
	ReleasePlanStatusWaitForApproval  ReleasePlanStatus = "wait_for_approval"
	ReleasePlanStatusExecuting        ReleasePlanStatus = "executing"
	ReleasePlanStatusSuccess          ReleasePlanStatus = "success"
	ReleasePlanStatusDenied           ReleasePlanStatus = "denied"
	ReleasePlanStatusCancel           ReleasePlanStatus = "cancel"
)

type ReleasePlanJobType string

const (
	ReleasePlanJobTypeText     ReleasePlanJobType = "text"
	ReleasePlanJobTypeWorkflow ReleasePlanJobType = "workflow"
)

type ReleasePlanJobStatus string

const (
	ReleasePlanJobStatusTodo    ReleasePlanJobStatus = "todo"
	ReleasePlanJobStatusRunning ReleasePlanJobStatus = "running"
	ReleasePlanJobStatusDone    ReleasePlanJobStatus = "done"
	ReleasePlanJobStatusSkipped ReleasePlanJobStatus = "skipped"
	ReleasePlanJobStatusFailed  ReleasePlanJobStatus = "failed"
)

type ReleasePlan struct {
	ID                  kernel.ReleasePlanID
	ProductID           kernel.ProductID
	ProductPlanID       *kernel.ProductPlanID
	SprintID            *kernel.SprintID
	Name                string
	Description         string
	ManagerID           kernel.MemberID
	ApproverID          *kernel.MemberID
	Status              ReleasePlanStatus
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ScheduleExecuteTime *time.Time
	ApprovalConfig      ApprovalConfig
	HookSettings        ReleasePlanHookSettings
	HookState           *ReleasePlanHookState
	Parameters          map[string]string
	CreatedBy           kernel.MemberID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int
	Jobs                []ReleasePlanJob
}

type ReleasePlanJob struct {
	ID            kernel.ReleasePlanJobID
	ReleasePlanID kernel.ReleasePlanID
	SortOrder     int
	Name          string
	Type          ReleasePlanJobType
	OwnerID       kernel.MemberID
	Status        ReleasePlanJobStatus
	TextContent   string
	TextFormat    string
	TextChecked   bool
	WorkflowRef   json.RawMessage
	ExecutionRef  json.RawMessage
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ReleasePlanAuditLog struct {
	ID            string
	ReleasePlanID kernel.ReleasePlanID
	ActorID       kernel.MemberID
	Action        string
	Detail        json.RawMessage
	CreatedAt     time.Time
}

func NewReleasePlan(productID kernel.ProductID, name, description string, managerID, createdBy kernel.MemberID, windowStart, windowEnd time.Time, approval ApprovalConfig) *ReleasePlan {
	now := time.Now().UTC()
	return &ReleasePlan{
		ID: kernel.NewReleasePlanID(), ProductID: productID, Name: name, Description: description,
		ManagerID: managerID, Status: ReleasePlanStatusPlanning,
		ReleaseWindowStart: windowStart, ReleaseWindowEnd: windowEnd,
		ApprovalConfig: approval, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
}

func (p *ReleasePlan) FinishPlanning() error {
	if p.Status != ReleasePlanStatusPlanning &&
		p.Status != ReleasePlanStatusWaitFinishPlanningExternalCheck {
		return domain.ErrInvalidReleasePlanTransition
	}
	if p.ApprovalConfig.Enabled {
		p.Status = ReleasePlanStatusWaitForApproval
	} else if p.HookSettings.EventEnabled(ReleasePlanHookEventBeforeExecute) {
		p.Status = ReleasePlanStatusWaitExecuteExternalCheck
	} else {
		p.Status = ReleasePlanStatusExecuting
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) Approve() error {
	if p.Status != ReleasePlanStatusWaitForApproval {
		return domain.ErrInvalidReleasePlanTransition
	}
	if p.HookSettings.EventEnabled(ReleasePlanHookEventBeforeExecute) {
		p.Status = ReleasePlanStatusWaitExecuteExternalCheck
	} else {
		p.Status = ReleasePlanStatusExecuting
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) Reject() error {
	if p.Status != ReleasePlanStatusWaitForApproval {
		return domain.ErrInvalidReleasePlanTransition
	}
	p.Status = ReleasePlanStatusDenied
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) ReopenPlanning() error {
	if p.Status != ReleasePlanStatusDenied {
		return domain.ErrInvalidReleasePlanTransition
	}
	p.Status = ReleasePlanStatusPlanning
	p.ApprovalConfig.Instance = nil
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) Cancel() error {
	switch p.Status {
	case ReleasePlanStatusPlanning, ReleasePlanStatusWaitForApproval, ReleasePlanStatusExecuting,
		ReleasePlanStatusWaitFinishPlanningExternalCheck, ReleasePlanStatusWaitExecuteExternalCheck,
		ReleasePlanStatusWaitAllDoneExternalCheck:
		p.Status = ReleasePlanStatusCancel
		p.HookState = nil
		p.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return domain.ErrInvalidReleasePlanTransition
	}
}

func (p *ReleasePlan) CanDelete() error {
	switch p.Status {
	case ReleasePlanStatusPlanning, ReleasePlanStatusDenied, ReleasePlanStatusCancel, ReleasePlanStatusSuccess,
		ReleasePlanStatusFinishPlanningExternalFailed, ReleasePlanStatusExecuteExternalFailed,
		ReleasePlanStatusAllDoneExternalFailed:
		return nil
	default:
		return domain.ErrInvalidReleasePlanTransition
	}
}

func (p *ReleasePlan) EnsureExecutingWindow(now time.Time) error {
	if p.Status != ReleasePlanStatusExecuting {
		return domain.ErrInvalidReleasePlanTransition
	}
	if now.Before(p.ReleaseWindowStart) || now.After(p.ReleaseWindowEnd) {
		return domain.ErrReleaseWindowClosed
	}
	return nil
}

func (p *ReleasePlan) sortedJobs() []ReleasePlanJob {
	jobs := append([]ReleasePlanJob(nil), p.Jobs...)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].SortOrder == jobs[j].SortOrder {
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		}
		return jobs[i].SortOrder < jobs[j].SortOrder
	})
	return jobs
}

func (p *ReleasePlan) CurrentJobIndex() int {
	for _, job := range p.sortedJobs() {
		switch job.Status {
		case ReleasePlanJobStatusDone, ReleasePlanJobStatusSkipped:
			continue
		default:
			for j := range p.Jobs {
				if p.Jobs[j].ID == job.ID {
					return j
				}
			}
		}
	}
	return -1
}

func (p *ReleasePlan) jobIndexByID(jobID kernel.ReleasePlanJobID) int {
	for i := range p.Jobs {
		if p.Jobs[i].ID == jobID {
			return i
		}
	}
	return -1
}

func (p *ReleasePlan) ensureCurrentJob(jobID kernel.ReleasePlanJobID) error {
	current := p.CurrentJobIndex()
	if current < 0 {
		return domain.ErrReleasePlanJobNotReady
	}
	if p.Jobs[current].ID != jobID {
		return domain.ErrReleasePlanJobNotReady
	}
	return nil
}

func (p *ReleasePlan) ParseWorkflowRef(job *ReleasePlanJob) (WorkflowRefSpec, error) {
	if len(job.WorkflowRef) == 0 {
		return WorkflowRefSpec{}, domain.ErrInvalidWorkflowRef
	}
	var spec WorkflowRefSpec
	if err := json.Unmarshal(job.WorkflowRef, &spec); err != nil {
		return WorkflowRefSpec{}, domain.ErrInvalidWorkflowRef
	}
	if spec.Mode == "" {
		spec.Mode = "reference"
	}
	if spec.WorkflowID == "" {
		return WorkflowRefSpec{}, domain.ErrInvalidWorkflowRef
	}
	switch spec.Mode {
	case "reference", "snapshot":
		return spec, nil
	default:
		return WorkflowRefSpec{}, domain.ErrInvalidWorkflowRef
	}
}

// ToRunInput builds the workflow run input; required lists the inputs the workflow needs,
// which are filled from planParams when the job does not override them.
func (spec WorkflowRefSpec) ToRunInput(planParams map[string]string, required []string) map[string]any {
	input := map[string]any{}
	params := spec.Parameters
	if branch := strings.TrimSpace(params["branch"]); branch != "" {
		input["branch"] = branch
	}
	if raw := strings.TrimSpace(params["_environment_overrides"]); raw != "" {
		var overrides map[string]string
		if err := json.Unmarshal([]byte(raw), &overrides); err == nil && len(overrides) > 0 {
			envMap := make(map[string]any, len(overrides))
			for jobName, envID := range overrides {
				if strings.TrimSpace(envID) != "" {
					envMap[jobName] = envID
				}
			}
			if len(envMap) > 0 {
				input["environment_overrides"] = envMap
			}
		}
	}
	if raw := strings.TrimSpace(params["_project_ids"]); raw != "" {
		var projectIDs []string
		if err := json.Unmarshal([]byte(raw), &projectIDs); err == nil {
			items := make([]any, 0, len(projectIDs))
			for _, projectID := range projectIDs {
				if id := strings.TrimSpace(projectID); id != "" {
					items = append(items, id)
				}
			}
			input["project_ids"] = items
		}
	}
	if overrides := parseStringMapParam(params["_branch_overrides"]); len(overrides) > 0 {
		input["branch_overrides"] = overrides
	}
	if overrides := parseStringMapParam(params["_ref_type_overrides"]); len(overrides) > 0 {
		input["ref_type_overrides"] = overrides
	}
	variables := map[string]any{}
	for key, value := range params {
		if !IsJobInputOverrideKey(key) {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			variables[key] = trimmed
		}
	}
	for _, key := range required {
		if _, ok := variables[key]; ok {
			continue
		}
		if value := strings.TrimSpace(planParams[key]); value != "" {
			variables[key] = value
		}
	}
	if len(variables) > 0 {
		input["variables"] = variables
	}
	return input
}

// parseStringMapParam decodes a JSON object parameter, dropping blank keys and values.
func parseStringMapParam(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		if k, v := strings.TrimSpace(key), strings.TrimSpace(value); k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func (p *ReleasePlan) MarkWorkflowJobRunning(jobID kernel.ReleasePlanJobID, runID kernel.ProductWorkflowRunID) error {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return err
	}
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusTodo {
		return domain.ErrInvalidReleasePlanJobTransition
	}
	ref, _ := json.Marshal(ExecutionRef{WorkflowRunID: runID.String()})
	job.ExecutionRef = ref
	job.Status = ReleasePlanJobStatusRunning
	job.UpdatedAt = time.Now().UTC()
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) CompleteWorkflowJob(jobID kernel.ReleasePlanJobID, success bool, errMsg string) (bool, error) {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return false, domain.ErrReleasePlanJobNotFound
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusRunning {
		return false, domain.ErrInvalidReleasePlanJobTransition
	}
	var ref ExecutionRef
	_ = json.Unmarshal(job.ExecutionRef, &ref)
	if !success {
		ref.ErrorMessage = errMsg
		job.Status = ReleasePlanJobStatusFailed
	} else {
		job.Status = ReleasePlanJobStatusDone
	}
	raw, _ := json.Marshal(ref)
	job.ExecutionRef = raw
	job.UpdatedAt = time.Now().UTC()
	if !success {
		p.UpdatedAt = time.Now().UTC()
		return false, nil
	}
	needsHook := p.maybeComplete()
	p.UpdatedAt = time.Now().UTC()
	return needsHook, nil
}

func (p *ReleasePlan) RetryWorkflowJob(jobID kernel.ReleasePlanJobID) error {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return err
	}
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusFailed {
		return domain.ErrInvalidReleasePlanJobTransition
	}
	job.Status = ReleasePlanJobStatusTodo
	job.ExecutionRef = nil
	job.UpdatedAt = time.Now().UTC()
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// FailWorkflowJobStart records that the current workflow job could not start, so it can be fixed and retried.
func (p *ReleasePlan) FailWorkflowJobStart(jobID kernel.ReleasePlanJobID, errMsg string) error {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusTodo {
		return domain.ErrInvalidReleasePlanJobTransition
	}
	ref, _ := json.Marshal(ExecutionRef{ErrorMessage: errMsg})
	job.ExecutionRef = ref
	job.Status = ReleasePlanJobStatusFailed
	now := time.Now().UTC()
	job.UpdatedAt = now
	p.UpdatedAt = now
	return nil
}

// EnsureJobEditable allows any edit while planning; once executing, only a failed
// workflow job's workflow reference (its run inputs) may change.
func (p *ReleasePlan) EnsureJobEditable(jobID kernel.ReleasePlanJobID, workflowRefOnly bool) error {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	switch p.Status {
	case ReleasePlanStatusPlanning:
		return nil
	case ReleasePlanStatusExecuting:
		job := p.Jobs[idx]
		if workflowRefOnly && job.Type == ReleasePlanJobTypeWorkflow && job.Status == ReleasePlanJobStatusFailed {
			return nil
		}
	}
	return domain.ErrInvalidReleasePlanTransition
}

func (p *ReleasePlan) InitApprovalInstance(token string, now time.Time) {
	if p.ApprovalConfig.Instance == nil {
		p.ApprovalConfig.Instance = &ApprovalInstance{}
	}
	p.ApprovalConfig.Instance.Token = token
	p.ApprovalConfig.Instance.Status = "pending"
	p.ApprovalConfig.Instance.SentAt = &now
}

func (p *ReleasePlan) ApproveByToken(note string, now time.Time) error {
	if p.Status != ReleasePlanStatusWaitForApproval {
		return domain.ErrInvalidReleasePlanTransition
	}
	if p.ApprovalConfig.Instance == nil || p.ApprovalConfig.Instance.Status != "pending" {
		return domain.ErrInvalidReleasePlanTransition
	}
	if err := p.Approve(); err != nil {
		return err
	}
	p.ApprovalConfig.Instance.Status = "approved"
	p.ApprovalConfig.Instance.ActedAt = &now
	p.ApprovalConfig.Instance.ActorNote = note
	p.UpdatedAt = now
	return nil
}

func (p *ReleasePlan) RejectByToken(note string, now time.Time) error {
	if p.Status != ReleasePlanStatusWaitForApproval {
		return domain.ErrInvalidReleasePlanTransition
	}
	if p.ApprovalConfig.Instance == nil || p.ApprovalConfig.Instance.Status != "pending" {
		return domain.ErrInvalidReleasePlanTransition
	}
	if err := p.Reject(); err != nil {
		return err
	}
	p.ApprovalConfig.Instance.Status = "rejected"
	p.ApprovalConfig.Instance.ActedAt = &now
	p.ApprovalConfig.Instance.ActorNote = note
	return nil
}

func (p *ReleasePlan) ScheduledExecutionPending(now time.Time) bool {
	if p.ScheduleExecuteTime == nil {
		return false
	}
	return now.Before(*p.ScheduleExecuteTime)
}

func (p *ReleasePlan) MarkTextJobDone(jobID kernel.ReleasePlanJobID, remark string) (bool, error) {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return false, err
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return false, err
	}
	for i := range p.Jobs {
		if p.Jobs[i].ID != jobID {
			continue
		}
		if p.Jobs[i].Type != ReleasePlanJobTypeText {
			return false, domain.ErrInvalidReleasePlanJobTransition
		}
		p.Jobs[i].TextChecked = true
		p.Jobs[i].Status = ReleasePlanJobStatusDone
		if strings.TrimSpace(remark) != "" {
			ref, _ := json.Marshal(ExecutionRef{DoneRemark: strings.TrimSpace(remark)})
			p.Jobs[i].ExecutionRef = ref
		}
		p.Jobs[i].UpdatedAt = time.Now().UTC()
		needsHook := p.maybeComplete()
		p.UpdatedAt = time.Now().UTC()
		return needsHook, nil
	}
	return false, domain.ErrReleasePlanJobNotFound
}

func (p *ReleasePlan) SkipJob(jobID kernel.ReleasePlanJobID) (bool, error) {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return false, err
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return false, err
	}
	for i := range p.Jobs {
		if p.Jobs[i].ID != jobID {
			continue
		}
		p.Jobs[i].Status = ReleasePlanJobStatusSkipped
		p.Jobs[i].UpdatedAt = time.Now().UTC()
		needsHook := p.maybeComplete()
		p.UpdatedAt = time.Now().UTC()
		return needsHook, nil
	}
	return false, domain.ErrReleasePlanJobNotFound
}

func (p *ReleasePlan) maybeComplete() bool {
	if len(p.Jobs) == 0 {
		if p.HookSettings.EventEnabled(ReleasePlanHookEventAllDone) {
			return true
		}
		p.Status = ReleasePlanStatusSuccess
		return false
	}
	for _, job := range p.Jobs {
		switch job.Status {
		case ReleasePlanJobStatusDone, ReleasePlanJobStatusSkipped:
			continue
		default:
			return false
		}
	}
	if p.HookSettings.EventEnabled(ReleasePlanHookEventAllDone) {
		return true
	}
	p.Status = ReleasePlanStatusSuccess
	return false
}
