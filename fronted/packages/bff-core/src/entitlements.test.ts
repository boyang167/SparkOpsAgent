import { describe, expect, it } from "vitest";
import { WorkspaceEntitlementsVMSchema } from "@sparkops/contracts/platform";
import { apiPathEntitlement, DEFAULT_ENTITLEMENTS, isModuleEnabled } from "./entitlements";
import { invalidateEntitlementsCache, resolveWorkspaceEntitlements } from "./entitlements-cache";

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
    "/api/v1/workspaces/ws/files////?sort=name",
    "/api/v1/workspaces/ws/namespaces",
    "/api/v1/workspaces/ws/namespaces/",
    "/api/v1/workspaces/ws/transfers",
    "/api/v1/workspaces/ws/transfers/",
    "/api/v1/workspaces/ws/directories",
    "/api/v1/workspaces/ws/directories/",
    "/api/v1/workspaces/ws/%66iles",
    "/api/v1/workspaces%2Fws/files",
  ])("maps workspace filesystem API %s to filesystem", (path) => {
    expect(apiPathEntitlement(path)).toBe("filesystem");
  });

  it.each([
    "/api/v1/workspaces/ws/files-legacy",
    "/api/v1/workspaces/ws/namespaces-old",
    "/api/v1/workspaces/ws/transfers-v2",
    "/api/v1/workspaces/ws/%66iles-legacy",
    "/api%2Fv1/workspaces/ws/files",
    "/api/v1/workspaces/ws/%ZZ",
    "/api/v1/workspaces/ws/files?sort=%ZZ",
    "/api/v1/files",
    "/api/v1/namespaces",
    "/api/v1/transfers",
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

  it("normalizes old upstream entitlements at the BFF boundary", async () => {
    invalidateEntitlementsCache();
    const legacy = {
      agent: false,
      knowledge: true,
      delivery: true,
      k8s: false,
      observability: true,
      mcp: false,
      plugins: [],
    };
    const normalized = await resolveWorkspaceEntitlements(
      "workspace-1",
      async () => legacy as never,
      { fresh: true },
    );
    expect(normalized).toEqual({ ...DEFAULT_ENTITLEMENTS, ...legacy, filesystem: true });

    const disabled = await resolveWorkspaceEntitlements(
      "workspace-2",
      async () => ({ ...legacy, filesystem: false }) as never,
      { fresh: true },
    );
    expect(disabled.filesystem).toBe(false);
    expect(Object.keys(disabled).sort()).toEqual(Object.keys(DEFAULT_ENTITLEMENTS).sort());
  });
});
