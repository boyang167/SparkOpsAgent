import { describe, expect, it } from "vitest";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import {
  buildEnvironmentDeployRows,
  environmentStatusLabel,
  environmentStatusTone,
  environmentVersionLabel,
  isProjectInDeployPhase,
  isRunOnline,
  onlineEnvironmentRows,
  type EnvironmentClusterRef,
} from "./project-environment-status";

describe("isProjectInDeployPhase", () => {
  it("returns true when review is approved", () => {
    expect(isProjectInDeployPhase({ review_status: "approved" })).toBe(true);
  });

  it("returns false before approval", () => {
    expect(isProjectInDeployPhase({ review_status: "pending" })).toBe(false);
  });
});

describe("buildEnvironmentDeployRows", () => {
  const clusters: EnvironmentClusterRef[] = [
    { id: "build", cluster_name: "build", is_build_env: true, priority_id: 0 },
    { id: "dev", cluster_name: "dev", is_build_env: false, priority_id: 1 },
    { id: "uat", cluster_name: "uat", is_build_env: false, priority_id: 2 },
    { id: "prod", cluster_name: "prod", is_build_env: false, priority_id: 3 },
  ];

  it("maps latest run per deploy cluster and skips build cluster", () => {
    const runs = [
      { id: "r2", cluster_id: "prod", status: "succeeded", image_tag: "v2", git_ref_value: "main" },
      { id: "r1", cluster_id: "dev", status: "failed", image_tag: "v1", git_ref_value: "main" },
      { id: "r0", cluster_id: "dev", status: "succeeded", image_tag: "v0", git_ref_value: "main" },
    ] as DeploymentRunVM[];

    expect(buildEnvironmentDeployRows(clusters, runs)).toEqual([
      { clusterId: "dev", clusterName: "dev", latest: runs[1] },
      { clusterId: "uat", clusterName: "uat", latest: null },
      { clusterId: "prod", clusterName: "prod", latest: runs[0] },
    ]);
  });

  it("shows all deploy environments sorted by priority", () => {
    expect(buildEnvironmentDeployRows(clusters, [])).toEqual([
      { clusterId: "dev", clusterName: "dev", latest: null },
      { clusterId: "uat", clusterName: "uat", latest: null },
      { clusterId: "prod", clusterName: "prod", latest: null },
    ]);
  });
});

describe("environmentVersionLabel", () => {
  it("prefers git ref and image tag together", () => {
    expect(
      environmentVersionLabel({
        id: "run",
        image_tag: "20260913-abc",
        git_ref_value: "main",
      } as DeploymentRunVM),
    ).toBe("main · 20260913-abc");
  });

  it("deduplicates when ref and tag are the same", () => {
    expect(
      environmentVersionLabel({
        id: "run",
        image_tag: "v1.0.1",
        git_ref_value: "v1.0.1",
      } as DeploymentRunVM),
    ).toBe("v1.0.1");
  });
});

describe("environmentStatusLabel", () => {
  it("shows undeployed when no run exists", () => {
    expect(environmentStatusLabel(null)).toBe("未部署");
  });
});

describe("undeploy runs", () => {
  const undeployed = {
    id: "u1", cluster_id: "dev", status: "succeeded", run_type: "undeploy", image_tag: "v1", git_ref_value: "main",
  } as DeploymentRunVM;

  it("treats succeeded undeploy as offline", () => {
    expect(isRunOnline(undeployed)).toBe(false);
    expect(environmentStatusLabel(undeployed)).toBe("已下线");
    expect(environmentStatusTone(undeployed)).toBe("neutral");
    expect(environmentVersionLabel(undeployed)).toBe("—");
  });

  it("labels running and failed undeploy", () => {
    expect(environmentStatusLabel({ ...undeployed, status: "deploying" })).toBe("下线中");
    expect(environmentStatusTone({ ...undeployed, status: "deploying" })).toBe("running");
    expect(environmentStatusLabel({ ...undeployed, status: "failed" })).toBe("下线失败");
    expect(environmentStatusTone({ ...undeployed, status: "failed" })).toBe("failed");
  });

  it("keeps normal succeeded run online", () => {
    const run = { ...undeployed, run_type: "normal" } as DeploymentRunVM;
    expect(isRunOnline(run)).toBe(true);
    expect(onlineEnvironmentRows([
      { clusterId: "dev", clusterName: "dev", latest: run },
      { clusterId: "uat", clusterName: "uat", latest: undeployed },
      { clusterId: "prod", clusterName: "prod", latest: null },
    ]).map((row) => row.clusterId)).toEqual(["dev"]);
  });
});
