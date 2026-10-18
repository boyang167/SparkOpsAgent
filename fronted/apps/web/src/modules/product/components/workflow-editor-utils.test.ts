import { describe, expect, it } from "vitest";
import { defaultJob, upstreamBuildJobs, type EditorStage } from "./workflow-editor-utils";

const stages: EditorStage[] = [
  {
    name: "构建",
    parallel: false,
    exec_mode: "auto",
    jobs: [
      { name: "build-api", type: "sparkops-build", spec: {} },
      { name: "notify-1", type: "notify", spec: {} },
    ],
  },
  {
    name: "部署",
    parallel: false,
    exec_mode: "auto",
    jobs: [
      { name: "build-web", type: "sparkops-build", spec: {} },
      { name: "deploy-dev", type: "sparkops-deploy", spec: {} },
      { name: "build-later", type: "sparkops-build", spec: {} },
    ],
  },
];

describe("upstreamBuildJobs", () => {
  it("lists build jobs before the current deploy job", () => {
    expect(upstreamBuildJobs(stages, 1, 1).map((job) => job.name)).toEqual(["build-api", "build-web"]);
  });

  it("does not include builds in later stages or later jobs", () => {
    expect(upstreamBuildJobs(stages, 0, 1).map((job) => job.name)).toEqual(["build-api"]);
    expect(upstreamBuildJobs(stages, 1, 0)).toEqual([{ name: "build-api", type: "sparkops-build", spec: {} }]);
  });
});

describe("defaultJob", () => {
  it("creates an environment sync job", () => {
    expect(defaultJob("sparkops-environment-sync", 3)).toMatchObject({
      name: "environment-sync-3",
      type: "sparkops-environment-sync",
      spec: { source_environment_id: "", target_environment_id: "", projects: [] },
    });
  });
});
