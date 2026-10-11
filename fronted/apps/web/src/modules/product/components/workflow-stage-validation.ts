import { parseCreateVersionJobSpec } from "./create-version-job-spec";
import { parseDeployJobSpec } from "./deploy-job-spec";
import {
  upstreamBuildJobs,
  upstreamVersionSourceJobs,
  type EditorStage,
} from "./workflow-editor-utils";

export interface WorkflowValidationIssue {
  stageName: string;
  jobName: string;
  message: string;
}

export function validateWorkflowStages(stages: EditorStage[]): WorkflowValidationIssue[] {
  const issues: WorkflowValidationIssue[] = [];

  for (let stageIndex = 0; stageIndex < stages.length; stageIndex++) {
    const stage = stages[stageIndex];
    const stageName = stage.name?.trim() || `阶段 ${stageIndex + 1}`;

    for (let jobIndex = 0; jobIndex < stage.jobs.length; jobIndex++) {
      const job = stage.jobs[jobIndex];
      const jobName = job.name?.trim() || "未命名任务";

      if (job.type === "sparkops-create-version") {
        const parsed = parseCreateVersionJobSpec(job.spec);
        const allowed = upstreamBuildJobs(stages, stageIndex, jobIndex).map((item) => item.name);
        if (!parsed.jobName.trim()) {
          issues.push({
            stageName,
            jobName,
            message: "未选择上游构建任务",
          });
        } else if (!allowed.includes(parsed.jobName)) {
          issues.push({
            stageName,
            jobName,
            message: `上游构建任务「${parsed.jobName}」无效或不在当前任务之前`,
          });
        }
        continue;
      }

      if (job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version") {
        const parsed = parseDeployJobSpec(job.spec, job.type);
        const source = typeof job.spec?.source === "string" ? job.spec.source : "fromjob";
        if (source !== "fromjob") continue;

        const allowed =
          parsed.artifactSource === "version"
            ? upstreamVersionSourceJobs(stages, stageIndex, jobIndex).map((item) => item.name)
            : upstreamBuildJobs(stages, stageIndex, jobIndex).map((item) => item.name);

        if (!parsed.jobName.trim()) {
          issues.push({
            stageName,
            jobName,
            message: parsed.artifactSource === "version" ? "未选择上游版本/构建任务" : "未选择上游构建任务",
          });
        } else if (!allowed.includes(parsed.jobName)) {
          issues.push({
            stageName,
            jobName,
            message: `上游任务「${parsed.jobName}」无效或不在当前任务之前`,
          });
        }
      }
    }
  }

  return issues;
}

export function formatWorkflowValidationErrors(issues: WorkflowValidationIssue[]): string {
  return issues.map((item) => `「${item.stageName} / ${item.jobName}」${item.message}`).join("\n");
}
