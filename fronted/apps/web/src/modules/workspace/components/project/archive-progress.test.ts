import { describe, expect, it } from "vitest";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import { archiveSteps } from "./archive-progress";

const pending = [
  { clusterId: "dev", clusterName: "DEV" },
  { clusterId: "uat", clusterName: "UAT" },
];
const startedAt = "2026-10-11T02:00:00Z";

function run(partial: Partial<DeploymentRunVM>): DeploymentRunVM {
  return {
    id: "r", cluster_id: "dev", run_type: "undeploy", status: "succeeded",
    created_at: "2026-10-11T02:00:05Z", error_message: "", ...partial,
  } as DeploymentRunVM;
}

describe("archiveSteps", () => {
  it("marks clusters waiting before any undeploy run", () => {
    expect(archiveSteps(pending, [], startedAt).map((s) => s.state)).toEqual(["waiting", "waiting"]);
  });

  it("tracks running, done and failed undeploy runs created after start", () => {
    const runs = [
      run({ id: "u2", cluster_id: "uat", status: "failed", error_message: "forbidden", created_at: "2026-10-11T02:01:00Z" }),
      run({ id: "u1", cluster_id: "dev", status: "succeeded" }),
    ];
    const steps = archiveSteps(pending, runs, startedAt);
    expect(steps.map((s) => s.state)).toEqual(["done", "failed"]);
    expect(steps[1].error).toBe("forbidden");
  });

  it("ignores undeploy runs created before archive started", () => {
    const runs = [run({ created_at: "2026-10-11T01:59:00Z" })];
    expect(archiveSteps(pending, runs, startedAt)[0].state).toBe("waiting");
  });

  it("shows running for active undeploy", () => {
    const runs = [run({ status: "deploying" })];
    expect(archiveSteps(pending, runs, startedAt)[0].state).toBe("running");
  });
});
