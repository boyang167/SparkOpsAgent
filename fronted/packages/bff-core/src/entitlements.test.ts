import { describe, expect, it } from "vitest";
import { apiPathEntitlement } from "./entitlements";

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
});
