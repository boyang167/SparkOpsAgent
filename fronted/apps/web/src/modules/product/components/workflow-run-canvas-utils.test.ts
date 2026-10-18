import { describe, expect, it } from "vitest";
import type { ProductWorkflowRunVM } from "@sparkops/contracts/product";
import { buildPipelineStages, pickDefaultTask, runElapsedSeconds } from "./workflow-run-canvas-utils";

const run: ProductWorkflowRunVM = {
  id: "run-1",
  workflow_id: "wf-1",
  product_id: "prod-1",
  status: "running",
  current_stage: 0,
  started_at: "2026-09-22T12:00:00+08:00",
  stage_runs: [
    {
      name: "构建",
      status: "running",
      jobs: [
        {
          name: "build-1",
          type: "sparkops-build",
          status: "running",
          builds: [
            { project_id: "p1", branch: "main", status: "success", build_id: "b1" },
            { project_id: "p2", branch: "dev", status: "running", build_id: "b2" },
          ],
        },
      ],
    },
    {
      name: "部署",
      status: "pending",
      jobs: [{ name: "deploy-1", type: "sparkops-deploy", status: "pending", deploys: [] }],
    },
  ],
};

function workflowRunToPipelineTasks(run: ProductWorkflowRunVM) {
  return buildPipelineStages(run, {}).flatMap((stage) => stage.tasks);
}

describe("buildPipelineStages", () => {
  it("renders every environment sync project as a deploy task", () => {
    const syncRun: ProductWorkflowRunVM = {
      id: "run-sync",
      workflow_id: "wf-1",
      product_id: "prod-1",
      status: "failed",
      current_stage: 0,
      stage_runs: [
        {
          name: "同步",
          status: "failed",
          jobs: [
            {
              name: "promote",
              type: "sparkops-environment-sync",
              status: "failed",
              deploys: [
                {
                  project_id: "p1",
                  environment_id: "prod",
                  source_environment_id: "dev",
                  sync_image: true,
                  sync_yaml: false,
                  first_sync: false,
                  status: "success",
                  deployment_run_id: "dr-1",
                  image_repository: "registry/app",
                  image_tag: "v1",
                },
                {
                  project_id: "p2",
                  environment_id: "prod",
                  source_environment_id: "dev",
                  sync_image: true,
                  sync_yaml: true,
                  first_sync: true,
                  status: "failed",
                  error_message: "deployment already running",
                },
              ],
            },
          ],
        },
      ],
    };
    const tasks = workflowRunToPipelineTasks(syncRun);
    expect(tasks.filter((task) => task.jobType === "sparkops-environment-sync")).toMatchObject([
      {
        projectId: "p1",
        kind: "deploy",
        sourceEnvironmentId: "dev",
        environmentId: "prod",
        syncImage: true,
        syncYaml: false,
        firstSync: false,
      },
      {
        projectId: "p2",
        kind: "deploy",
        status: "failed",
        firstSync: true,
        errorMessage: "deployment already running",
      },
    ]);
  });

  it("flattens build tasks per project", () => {
    const stages = buildPipelineStages(run, { p1: "backend", p2: "frontend" });
    expect(stages[0]?.tasks).toHaveLength(2);
    expect(stages[0]?.tasks[0]?.projectLabel).toBe("backend");
    expect(stages[0]?.tasks[1]?.kind).toBe("build");
  });

  it("maps create-version tasks to version kind", () => {
    const versionRun: ProductWorkflowRunVM = {
      ...run,
      stage_runs: [
        {
          name: "版本",
          status: "success",
          jobs: [
            {
              name: "create-version-1",
              type: "sparkops-create-version",
              status: "success",
              versions: [
                {
                  project_id: "p1",
                  version_id: "ver-1",
                  version_tag: "v1.0.0",
                  status: "success",
                },
              ],
            },
          ],
        },
      ],
    };
    const stages = buildPipelineStages(versionRun, { p1: "backend" });
    const task = stages[0]?.tasks[0];
    expect(task?.kind).toBe("version");
    expect(task?.refId).toBe("ver-1");
    expect(task?.versionTag).toBe("v1.0.0");
  });
});

describe("pickDefaultTask", () => {
  it("prefers running task", () => {
    const stages = buildPipelineStages(run, {});
    expect(pickDefaultTask(stages)?.refId).toBe("b2");
  });
});

describe("runElapsedSeconds", () => {
  it("computes duration from started_at", () => {
    const seconds = runElapsedSeconds(run, Date.parse("2026-09-22T12:00:06+08:00"));
    expect(seconds).toBe(6);
  });
});
