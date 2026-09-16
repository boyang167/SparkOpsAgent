package engine

import (
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

func StartApprovalJob(jobRun *model.JobRun, job model.WorkflowJob) {
	jobRun.Type = job.Type
	jobRun.Status = model.JobRunStatusRunning
	jobRun.Approval = &model.ApprovalRun{Status: model.ApprovalStatusPending}
}

func IsApprovalWaiting(jobRun model.JobRun) bool {
	return jobRun.Type == model.JobTypeSparkopsApproval &&
		jobRun.Approval != nil &&
		jobRun.Approval.Status == model.ApprovalStatusPending
}
