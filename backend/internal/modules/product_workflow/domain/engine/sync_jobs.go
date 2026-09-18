package engine

import (
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

// RunFreestyleJob marks freestyle jobs as succeeded synchronously (script logged for audit).
func RunFreestyleJob(jobRun *model.JobRun, job model.WorkflowJob) {
	script, _ := job.Spec["script"].(string)
	script = strings.TrimSpace(script)
	jobRun.Status = model.JobRunStatusSuccess
	if script == "" {
		jobRun.Message = "freestyle completed (empty script)"
		return
	}
	if len(script) > 500 {
		script = script[:500] + "…"
	}
	jobRun.Message = fmt.Sprintf("freestyle completed\n%s", script)
}

// RunNotifyJob marks notify jobs as succeeded synchronously.
func RunNotifyJob(jobRun *model.JobRun, job model.WorkflowJob) {
	channel, _ := job.Spec["channel"].(string)
	message, _ := job.Spec["message"].(string)
	if strings.TrimSpace(channel) == "" {
		channel = "in-app"
	}
	if strings.TrimSpace(message) == "" {
		message = "workflow notification"
	}
	jobRun.Status = model.JobRunStatusSuccess
	jobRun.Message = fmt.Sprintf("[%s] %s", channel, message)
}
