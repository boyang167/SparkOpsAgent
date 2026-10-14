import { describe, expect, it } from "vitest";
import type { ProductWorkflowVM } from "@sparkops/contracts/product";
import {
  buildEnvironmentOverrides,
  buildWorkflowJobParameters,
  defaultBranchForWorkflow,
  mergeWorkflowJobParameters,
  parseWorkflowJobBranch,
  parseWorkflowJobEnvironmentId,
  parseWorkflowJobProjectIds,
  resolveWorkflowDetails,
  workflowHasBuildJobs,
  workflowProjectIds,
} from "./release-plan-workflow-params";

const workflow: ProductWorkflowVM = {
  id: "wf-1",
  product_id: "p-1",
  name: "发布流水线",
  key: "release",
  status: "active",
  stage_count: 1,
  notifications: [],
  variables: [],
  triggers: [],
  stages: [
    {
      name: "部署",
      parallel: false,
      exec_mode: "auto",
      jobs: [
        { name: "deploy-dev", type: "sparkops-deploy", spec: { environment_id: "env-old" } },
        {
          name: "lint",
          type: "sparkops-build",
          spec: {
            branch: "develop",
            project_builds: [
              { project_id: "project-1", branch: "develop" },
              { project_id: "project-2", branch: "main" },
            ],
          },
        },
      ],
    },
  ],
};

describe("release-plan-workflow-params", () => {
  it("builds environment overrides for deploy jobs", () => {
    expect(buildEnvironmentOverrides(workflow, "env-new")).toEqual({ "deploy-dev": "env-new" });
  });

  it("stores environment_id and serialized overrides", () => {
    const params = buildWorkflowJobParameters(workflow, "env-new");
    expect(params.environment_id).toBe("env-new");
    expect(JSON.parse(params._environment_overrides ?? "{}")).toEqual({ "deploy-dev": "env-new" });
    expect(parseWorkflowJobEnvironmentId(params)).toBe("env-new");
  });

  it("detects build jobs and default branch", () => {
    expect(workflowHasBuildJobs(workflow)).toBe(true);
    expect(defaultBranchForWorkflow(workflow)).toBe("develop");
  });

  it("stores branch and merges partial updates", () => {
    const params = buildWorkflowJobParameters(workflow, "env-new", "feature/x");
    expect(params.branch).toBe("feature/x");
    expect(parseWorkflowJobBranch(params)).toBe("feature/x");

    const merged = mergeWorkflowJobParameters(params, workflow, { branch: "main" });
    expect(merged.branch).toBe("main");
    expect(merged.environment_id).toBe("env-new");

    const cleared = mergeWorkflowJobParameters(merged, workflow, { branch: null });
    expect(cleared.branch).toBeUndefined();
  });

  it("extracts projects and stores the selected project ids", () => {
    expect(workflowProjectIds(workflow)).toEqual(["project-1", "project-2"]);

    const params = buildWorkflowJobParameters(workflow, "env-new", "main", ["project-2"]);
    expect(parseWorkflowJobProjectIds(params)).toEqual(["project-2"]);

    const merged = mergeWorkflowJobParameters(params, workflow, { projectIds: ["project-1"] });
    expect(parseWorkflowJobProjectIds(merged)).toEqual(["project-1"]);
  });

  it("loads workflow details when the list item omits stages", async () => {
    const summary: ProductWorkflowVM = { ...workflow, stages: undefined };
    const loadedIds: string[] = [];

    const resolved = await resolveWorkflowDetails(summary, async (workflowId) => {
      loadedIds.push(workflowId);
      return workflow;
    });

    expect(loadedIds).toEqual(["wf-1"]);
    expect(workflowProjectIds(resolved)).toEqual(["project-1", "project-2"]);
  });
});
