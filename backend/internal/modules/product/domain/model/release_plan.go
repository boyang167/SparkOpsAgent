package model

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRefSpec struct {
	Mode       string            `json:"mode"`
	WorkflowID string            `json:"workflow_id"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

type ExecutionRef struct {
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	BuildID       string `json:"build_id,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
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

type ApprovalConfig struct {
	Enabled bool `json:"enabled"`
}

type ReleasePlan struct {
	ID                  kernel.ReleasePlanID
	ProductID           kernel.ProductID
	ProductPlanID       *kernel.ProductPlanID
	Name                string
	Description         string
	ManagerID           kernel.MemberID
	ApproverID          *kernel.MemberID
	Status              ReleasePlanStatus
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ScheduleExecuteTime *time.Time
	ApprovalConfig      ApprovalConfig
	CreatedBy           kernel.MemberID
	CreatedAt           time.Time
	UpdatedAt           time.Time
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
	if p.Status != ReleasePlanStatusPlanning {
		return domain.ErrInvalidReleasePlanTransition
	}
	if p.ApprovalConfig.Enabled {
		p.Status = ReleasePlanStatusWaitForApproval
	} else {
		p.Status = ReleasePlanStatusExecuting
		for i := range p.Jobs {
			if p.Jobs[i].Status == ReleasePlanJobStatusTodo {
				p.Jobs[i].Status = ReleasePlanJobStatusTodo
			}
		}
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *ReleasePlan) Approve() error {
	if p.Status != ReleasePlanStatusWaitForApproval {
		return domain.ErrInvalidReleasePlanTransition
	}
	p.Status = ReleasePlanStatusExecuting
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

func (p *ReleasePlan) Cancel() error {
	switch p.Status {
	case ReleasePlanStatusPlanning, ReleasePlanStatusWaitForApproval, ReleasePlanStatusExecuting:
		p.Status = ReleasePlanStatusCancel
		p.UpdatedAt = time.Now().UTC()
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
	if spec.Mode != "reference" || spec.WorkflowID == "" {
		return WorkflowRefSpec{}, domain.ErrInvalidWorkflowRef
	}
	return spec, nil
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

func (p *ReleasePlan) CompleteWorkflowJob(jobID kernel.ReleasePlanJobID, success bool, errMsg string) error {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusRunning {
		return domain.ErrInvalidReleasePlanJobTransition
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
		return nil
	}
	p.maybeComplete()
	p.UpdatedAt = time.Now().UTC()
	return nil
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

func (p *ReleasePlan) MarkTextJobDone(jobID kernel.ReleasePlanJobID) error {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return err
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	for i := range p.Jobs {
		if p.Jobs[i].ID != jobID {
			continue
		}
		if p.Jobs[i].Type != ReleasePlanJobTypeText {
			return domain.ErrInvalidReleasePlanJobTransition
		}
		p.Jobs[i].TextChecked = true
		p.Jobs[i].Status = ReleasePlanJobStatusDone
		p.Jobs[i].UpdatedAt = time.Now().UTC()
		p.maybeComplete()
		p.UpdatedAt = time.Now().UTC()
		return nil
	}
	return domain.ErrReleasePlanJobNotFound
}

func (p *ReleasePlan) SkipJob(jobID kernel.ReleasePlanJobID) error {
	if err := p.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return err
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	for i := range p.Jobs {
		if p.Jobs[i].ID != jobID {
			continue
		}
		p.Jobs[i].Status = ReleasePlanJobStatusSkipped
		p.Jobs[i].UpdatedAt = time.Now().UTC()
		p.maybeComplete()
		p.UpdatedAt = time.Now().UTC()
		return nil
	}
	return domain.ErrReleasePlanJobNotFound
}

func (p *ReleasePlan) maybeComplete() {
	if len(p.Jobs) == 0 {
		p.Status = ReleasePlanStatusSuccess
		return
	}
	for _, job := range p.Jobs {
		switch job.Status {
		case ReleasePlanJobStatusDone, ReleasePlanJobStatusSkipped:
			continue
		default:
			return
		}
	}
	p.Status = ReleasePlanStatusSuccess
}
