import { describe, expect, it } from "vitest";
import type { WorkflowStageVM } from "@sparkops/contracts/product";
import { buildWorkflowRunForm, defaultDeployEnvironmentId, defaultRunBranch, repoLabelFromGitUrl } from "./workflow-run-form";

describe("workflow-run-form", () => {
  it("parses git host/path into repo label", () => {
    expect(repoLabelFromGitUrl("https://git.example.com/group/zadig-training.git", "x")).toBe("group/zadig-training");
    expect(repoLabelFromGitUrl("", "backend")).toBe("backend");
  });

  it("groups build and deploy jobs by stage", () => {
    const form = buildWorkflowRunForm(
      [
        {
          name: "构建",
          parallel: false,
          exec_mode: "auto",
          jobs: [
            {
              name: "build-services",
              type: "sparkops-build",
              spec: {
                branch: "develop",
                project_builds: [
                  { project_id: "p1", build_name: "backend", git_url: "https://git.example.com/org/api.git" },
                ],
              },
            },
          ],
        },
        {
          name: "部署",
          parallel: false,
          exec_mode: "auto",
          jobs: [
            {
              name: "deploy-dev",
              type: "sparkops-deploy",
              spec: { environment_id: "env-1", job_name: "build-services" },
            },
          ],
        },
        {
          name: "审批",
          parallel: false,
          exec_mode: "auto",
          jobs: [{ name: "gate", type: "sparkops-approval", spec: {} }],
        },
      ],
      [{ id: "p1", name: "api", status: "active", display_name: "API" }],
    );

    expect(form.pills.map((item) => item.name)).toEqual(["构建", "部署", "审批"]);
    expect(form.stages[0].buildRows[0]).toMatchObject({
      projectName: "API",
      repoLabel: "org/api",
    });
    expect(form.stages[1].deployJobs[0]).toMatchObject({
      jobName: "deploy-dev",
      environmentId: "env-1",
      rows: [{ projectId: "p1", projectName: "API", buildKey: "build-services:p1", buildJobName: "build-services" }],
    });
    expect(form.stages[2].otherJobs[0]).toMatchObject({ jobName: "gate", typeLabel: "审批" });
  });

  it("defaults branch and environment from specs", () => {
    const stages: WorkflowStageVM[] = [
      {
        name: "构建",
        parallel: false,
        exec_mode: "auto",
        jobs: [{ name: "b", type: "sparkops-build", spec: { project_builds: [{ project_id: "p1", branch: "release" }] } }],
      },
      {
        name: "部署",
        parallel: false,
        exec_mode: "auto",
        jobs: [{ name: "d", type: "sparkops-deploy", spec: { environment_id: "env-9" } }],
      },
    ];
    expect(defaultRunBranch(stages)).toBe("release");
    expect(defaultDeployEnvironmentId(stages)).toBe("env-9");
  });

  it("skips empty stages", () => {
    expect(buildWorkflowRunForm([{ name: "空", parallel: false, exec_mode: "auto", jobs: [] }]).pills).toEqual([]);
  });

  it("fills git url and secret from workspace project when job spec omits them", () => {
    const form = buildWorkflowRunForm(
      [
        {
          name: "构建",
          parallel: false,
          exec_mode: "auto",
          jobs: [
            {
              name: "build-1",
              type: "sparkops-build",
              spec: { project_builds: [{ project_id: "p1", build_name: "backend" }] },
            },
          ],
        },
      ],
      [],
      [{ id: "p1", git_url: "https://git.example.com/org/api.git", secret_id: "sec-1" }],
    );
    expect(form.stages[0].buildRows[0]).toMatchObject({
      gitUrl: "https://git.example.com/org/api.git",
      secretId: "sec-1",
      repoLabel: "org/api",
    });
  });
});
