package engine

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

func StartApprovalJob(jobRun *model.JobRun, job model.WorkflowJob) {
	spec := ParseApprovalSpec(job.Spec)
	required := spec.RequiredApprovals
	if required <= 0 {
		required = 1
	}
	jobRun.Type = job.Type
	jobRun.Status = model.JobRunStatusRunning
	jobRun.Approval = &model.ApprovalRun{
		Status:            model.ApprovalStatusPending,
		StartedAt:         time.Now().UTC().Format(time.RFC3339),
		TimeoutMinutes:    spec.TimeoutMinutes,
		RequiredApprovals: required,
		ApprovedBy:        []string{},
	}
}

func IsApprovalWaiting(jobRun model.JobRun) bool {
	return jobRun.Type == model.JobTypeSparkopsApproval &&
		jobRun.Approval != nil &&
		jobRun.Approval.Status == model.ApprovalStatusPending
}

func approvalTimedOut(approval *model.ApprovalRun) bool {
	if approval == nil || approval.StartedAt == "" || approval.TimeoutMinutes <= 0 {
		return false
	}
	started, err := time.Parse(time.RFC3339, approval.StartedAt)
	if err != nil {
		return false
	}
	return time.Since(started) > time.Duration(approval.TimeoutMinutes)*time.Minute
}
