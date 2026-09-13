import { describe, expect, it } from "vitest";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { computeProjectProgress, deploymentStatusLabel } from "./project-progress";

const approvedProject = (): ProjectVM =>
  ({
    id: "p1",
    name: "test-repo",
    review_status: "approved",
    git_url: "https://git.example/repo.git",
    git_ref_value: "main",
    dockerfile_path: "./Dockerfile",
    template_id: "tpl-1",
    resources: {},
  }) as ProjectVM;

const run = (status: DeploymentRunVM["status"]): DeploymentRunVM =>
  ({
    id: "run-1",
    status,
    created_at: "2026-09-13T10:00:00Z",
  }) as DeploymentRunVM;

describe("deploymentStatusLabel", () => {
  it("maps succeeded to 已部署", () => {
    expect(deploymentStatusLabel(run("succeeded"))).toBe("已部署");
  });

  it("maps active statuses to 部署中", () => {
    expect(deploymentStatusLabel(run("deploying"))).toBe("部署中");
  });
});

describe("computeProjectProgress", () => {
  it("shows 待部署 when approved but never deployed", () => {
    const { statusLabel } = computeProjectProgress(approvedProject());
    expect(statusLabel).toBe("待部署");
  });

  it("shows 已部署 after successful deployment", () => {
    const { statusLabel, steps } = computeProjectProgress(approvedProject(), {
      latest: run("succeeded"),
      total: 1,
    });
    expect(statusLabel).toBe("已部署");
    expect(steps.find((s) => s.key === "approved")?.subtitle).toBe("已部署");
  });
});
