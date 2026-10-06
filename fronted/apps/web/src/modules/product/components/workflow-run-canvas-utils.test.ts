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

describe("buildPipelineStages", () => {
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
