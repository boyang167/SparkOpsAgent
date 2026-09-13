import { describe, expect, it } from "vitest";
import type { BuildVM } from "@sparkops/contracts/delivery";
import { buildStatusLabel, buildSummaryState } from "./build-summary";

describe("buildSummaryState", () => {
  it("uses build list length as count", () => {
    const builds = [
      { id: "latest", status: "success", started_at: "2026-01-01T00:00:00Z" },
      { id: "older", status: "failed", started_at: "2025-12-31T00:00:00Z" },
    ] as BuildVM[];
    expect(buildSummaryState(builds, false, null)).toMatchObject({
      count: "2",
      latest: builds[0],
      unavailable: false,
    });
  });

  it("does not disguise API failures as zero builds", () => {
    expect(buildSummaryState(null, false, "network failed")).toEqual({
      count: "不可用",
      latest: null,
      unavailable: true,
    });
  });
});

describe("buildStatusLabel", () => {
  it("maps known statuses to Chinese labels", () => {
    expect(buildStatusLabel("success")).toBe("成功");
    expect(buildStatusLabel("running")).toBe("构建中");
  });
});
