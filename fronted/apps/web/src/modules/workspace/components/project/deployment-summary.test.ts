import { describe, expect, it } from "vitest";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import { deploymentSummaryState } from "./deployment-summary";

describe("deploymentSummaryState", () => {
  it("uses backend total instead of loaded item count", () => {
    const page = {
      items: [{ id: "latest", status: "succeeded" }],
      total: 42,
      page: 1,
      page_size: 20,
    } as DeploymentRunListVM;
    expect(deploymentSummaryState(page, false, null)).toMatchObject({
      count: "42",
      latest: page.items[0],
      unavailable: false,
    });
  });

  it("does not disguise API failures as zero deployments", () => {
    expect(deploymentSummaryState(null, false, "network failed")).toEqual({
      count: "不可用",
      latest: null,
      unavailable: true,
    });
  });
});
