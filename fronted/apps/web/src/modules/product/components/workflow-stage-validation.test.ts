import { describe, expect, it } from "vitest";
import { pickDefaultUpstreamBuildJob, prepareNewJob, type EditorStage } from "./workflow-editor-utils";
import { formatWorkflowValidationErrors, validateWorkflowStages } from "./workflow-stage-validation";

const stages: EditorStage[] = [
  {
    name: "构建",
    parallel: true,
    exec_mode: "auto",
    jobs: [{ name: "build-1", type: "sparkops-build", spec: { project_builds: [] } }],
  },
  {
    name: "版本",
    parallel: false,
    exec_mode: "auto",
    jobs: [
      {
        name: "create-version-2",
        type: "sparkops-create-version",
        spec: { version_tag: "{{version}}", source: "fromjob", job_name: "" },
      },
    ],
  },
];

describe("validateWorkflowStages", () => {
  it("flags missing upstream build job on create-version", () => {
    const issues = validateWorkflowStages(stages);
    expect(issues.some((item) => item.jobName === "create-version-2" && item.message.includes("上游构建任务"))).toBe(true);
    expect(formatWorkflowValidationErrors(issues)).toContain("版本 / create-version-2");
  });

  it("flags invalid environment sync job", () => {
    const invalid: EditorStage[] = [
      {
        name: "同步",
        parallel: false,
        exec_mode: "auto",
        jobs: [
          {
            name: "sync-1",
            type: "sparkops-environment-sync",
            spec: { source_environment_id: "env-a", target_environment_id: "env-a", projects: [] },
          },
        ],
      },
    ];
    const issues = validateWorkflowStages(invalid);
    expect(issues.some((item) => item.jobName === "sync-1" && item.message.includes("不能相同"))).toBe(true);
    expect(formatWorkflowValidationErrors(issues)).toContain("同步 / sync-1");
  });

  it("passes when environment sync job is configured", () => {
    const valid: EditorStage[] = [
      {
        name: "同步",
        parallel: false,
        exec_mode: "auto",
        jobs: [
          {
            name: "sync-1",
            type: "sparkops-environment-sync",
            spec: {
              source_environment_id: "env-a",
              target_environment_id: "env-b",
              projects: [{ project_id: "p1", sync_image: true, sync_yaml: false }],
            },
          },
        ],
      },
    ];
    expect(validateWorkflowStages(valid)).toEqual([]);
  });

  it("passes when create-version references upstream build", () => {
    const valid: EditorStage[] = [
      stages[0],
      {
        ...stages[1],
        jobs: [
          {
            name: "create-version-2",
            type: "sparkops-create-version",
            spec: { version_tag: "{{version}}", source: "fromjob", job_name: "build-1" },
          },
        ],
      },
    ];
    expect(validateWorkflowStages(valid)).toEqual([]);
  });
});

describe("prepareNewJob", () => {
  it("auto binds nearest upstream build job for create-version", () => {
    const job = prepareNewJob("sparkops-create-version", stages, 1, 0);
    expect(job.spec?.job_name).toBe("build-1");
  });

  it("pickDefaultUpstreamBuildJob returns latest upstream build", () => {
    expect(pickDefaultUpstreamBuildJob(stages, 1, 0)).toBe("build-1");
  });
});
