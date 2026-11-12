import { describe, expect, it } from "vitest";
import { WorkspaceEntitlementsVMSchema } from "@sparkops/contracts/platform";
import { apiPathEntitlement, DEFAULT_ENTITLEMENTS, isModuleEnabled } from "./entitlements";

describe("apiPathEntitlement", () => {
  it.each([
    "/api/v1/projects/p1/deployments",
    "/api/v1/projects/p1/deployments/options",
    "/api/v1/projects/p1/deployments/rollback",
    "/api/v1/deployments/run-1",
    "/api/v1/deployments/run-1/stream",
  ])("maps project release API %s to delivery", (path) => {
    expect(apiPathEntitlement(path)).toBe("delivery");
  });

  it("keeps k8s workload deployments under k8s", () => {
    expect(apiPathEntitlement("/api/v1/workspaces/ws/k8s/projects/p1/deployments")).toBe("k8s");
  });

  it.each([
    "/api/v1/projects/p1/pods",
    "/api/v1/projects/p1/pods/pod-1/logs",
    "/api/v1/projects/p1/pods/pod-1/terminal-ticket",
    "/api/v1/projects/p1/pods/pod-1/terminal/stream",
    "/api/v1/projects/p1/runtime/overview",
    "/api/v1/workspaces/ws/projects/p1/pods",
    "/api/v1/workspaces/ws/projects/p1/pods/pod-1/terminal-ticket",
    "/api/v1/workspaces/ws/projects/p1/runtime/overview",
  ])("maps project runtime API %s to delivery", (path) => {
    expect(apiPathEntitlement(path)).toBe("delivery");
  });

  it("keeps Kubernetes plugin pod APIs under k8s", () => {
    expect(apiPathEntitlement("/api/v1/workspaces/ws/k8s/projects/p1/pods")).toBe("k8s");
  });

  it.each([
    "/api/v1/workspaces/ws/files",
    "/api/v1/workspaces/ws/files?sort=name",
    "/api/v1/workspaces/ws/files/",
    "/api/v1/workspaces/ws/namespaces",
    "/api/v1/workspaces/ws/transfers/",
  ])("maps workspace filesystem API %s to filesystem", (path) => {
    expect(apiPathEntitlement(path)).toBe("filesystem");
  });

  it.each([
    "/api/v1/workspaces/ws/files-legacy",
    "/api/v1/workspaces/ws/namespaces-old",
    "/api/v1/workspaces/ws/transfers-v2",
    "/api/v1/files",
    "/api/v1/orgs/org/workspaces/ws/files",
  ])("does not map non-filesystem API %s", (path) => {
    expect(apiPathEntitlement(path)).toBe("");
  });

  it("defaults filesystem to enabled and preserves explicit false", () => {
    expect(DEFAULT_ENTITLEMENTS.filesystem).toBe(true);
    expect(isModuleEnabled({ ...DEFAULT_ENTITLEMENTS, filesystem: false }, "filesystem")).toBe(false);
    const legacy = { ...DEFAULT_ENTITLEMENTS };
    delete (legacy as Partial<typeof legacy>).filesystem;
    expect(WorkspaceEntitlementsVMSchema.parse(legacy).filesystem).toBe(true);
    expect(WorkspaceEntitlementsVMSchema.parse({ ...legacy, filesystem: false }).filesystem).toBe(false);
  });
});
