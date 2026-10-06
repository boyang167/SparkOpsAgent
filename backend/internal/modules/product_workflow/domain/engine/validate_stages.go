package engine

import (
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func validateStageJobRefs(stages []model.WorkflowStage) error {
	for si, stage := range stages {
		for ji, job := range stage.Jobs {
			spec := job.Spec
			if spec == nil {
				spec = map[string]any{}
			}
			switch job.Type {
			case model.JobTypeSparkopsCreateVersion:
				if err := requireUpstreamJobName(job.Name, jobSpecString(spec, "job_name"), upstreamBuildJobNames(stages, si, ji), "上游构建任务"); err != nil {
					return err
				}
				if strings.TrimSpace(jobSpecString(spec, "version_tag")) == "" {
					return invalidJobRef(job.Name, "version_tag 不能为空")
				}
			case model.JobTypeSparkopsDeploy, model.JobTypeSparkopsDeployVersion:
				source := jobSpecString(spec, "source")
				if source != "" && source != "fromjob" {
					continue
				}
				ref := jobSpecString(spec, "job_name")
				if strings.TrimSpace(ref) == "" {
					return invalidJobRef(job.Name, "未配置上游任务 job_name")
				}
				allowed := deployUpstreamJobNames(job.Type, spec, stages, si, ji)
				if !containsName(allowed, ref) {
					return invalidJobRef(job.Name, fmt.Sprintf("上游任务 %q 无效或不在当前任务之前", ref))
				}
			}
		}
	}
	return nil
}

func jobSpecString(spec map[string]any, key string) string {
	if spec == nil {
		return ""
	}
	raw, _ := spec[key].(string)
	return strings.TrimSpace(raw)
}

func invalidJobRef(jobName, detail string) error {
	label := strings.TrimSpace(jobName)
	if label == "" {
		label = "未命名任务"
	}
	return kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", fmt.Sprintf("任务 %s：%s", label, detail))
}

func requireUpstreamJobName(jobName, ref string, allowed []string, label string) error {
	if strings.TrimSpace(ref) == "" {
		return invalidJobRef(jobName, fmt.Sprintf("未配置%s job_name", label))
	}
	if !containsName(allowed, ref) {
		return invalidJobRef(jobName, fmt.Sprintf("%s %q 无效或不在当前任务之前", label, ref))
	}
	return nil
}

func containsName(items []string, name string) bool {
	for _, item := range items {
		if item == name {
			return true
		}
	}
	return false
}

func upstreamBuildJobNames(stages []model.WorkflowStage, stageIndex, jobIndex int) []string {
	out := make([]string, 0)
	for si := 0; si < len(stages) && si <= stageIndex; si++ {
		jobs := stages[si].Jobs
		limit := len(jobs)
		if si == stageIndex {
			limit = jobIndex
		}
		for ji := 0; ji < limit; ji++ {
			job := jobs[ji]
			if job.Type == model.JobTypeSparkopsBuild && strings.TrimSpace(job.Name) != "" {
				out = append(out, strings.TrimSpace(job.Name))
			}
		}
	}
	return out
}

func upstreamCreateVersionJobNames(stages []model.WorkflowStage, stageIndex, jobIndex int) []string {
	out := make([]string, 0)
	for si := 0; si < len(stages) && si <= stageIndex; si++ {
		jobs := stages[si].Jobs
		limit := len(jobs)
		if si == stageIndex {
			limit = jobIndex
		}
		for ji := 0; ji < limit; ji++ {
			job := jobs[ji]
			if job.Type == model.JobTypeSparkopsCreateVersion && strings.TrimSpace(job.Name) != "" {
				out = append(out, strings.TrimSpace(job.Name))
			}
		}
	}
	return out
}

func deployUpstreamJobNames(jobType string, spec map[string]any, stages []model.WorkflowStage, stageIndex, jobIndex int) []string {
	if deployArtifactSource(jobType, spec) == "version" {
		out := upstreamCreateVersionJobNames(stages, stageIndex, jobIndex)
		for _, name := range upstreamBuildJobNames(stages, stageIndex, jobIndex) {
			if !containsName(out, name) {
				out = append(out, name)
			}
		}
		return out
	}
	return upstreamBuildJobNames(stages, stageIndex, jobIndex)
}
