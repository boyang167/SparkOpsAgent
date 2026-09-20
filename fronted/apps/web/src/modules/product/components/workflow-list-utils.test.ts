import { describe, expect, it } from "vitest";
import { computeRunStats, formatLastRun, paginationPages, stagePathLabels, successRateTone } from "./workflow-list-utils";

describe("workflow-list-utils", () => {
  it("builds stage path labels from first job type", () => {
    expect(
      stagePathLabels([
        { name: "阶段1", jobs: [{ name: "build-1", type: "sparkops-build", spec: {} }] },
        { name: "阶段2", jobs: [{ name: "deploy-1", type: "sparkops-deploy", spec: {} }] },
      ]),
    ).toEqual(["构建", "部署"]);
  });

  it("computes sparkline and success rate", () => {
    const stats = computeRunStats([
      { id: "1", status: "success", current_stage: 0, created_at: "2026-09-01T10:00:00Z" },
      { id: "2", status: "failed", current_stage: 0, created_at: "2026-09-01T11:00:00Z" },
      { id: "3", status: "success", current_stage: 0, created_at: "2026-09-01T12:00:00Z" },
    ]);
    expect(stats.successRate).toBe(66.67);
    expect(stats.sparkline.at(-1)).toBe("success");
    expect(stats.lastRunAt).toBe("2026-09-01T12:00:00Z");
  });

  it("formats last run timestamp", () => {
    expect(formatLastRun("2026-09-01T11:39:00Z")).toMatch(/^\d{2}-\d{2} \d{2}:\d{2}$/);
    expect(formatLastRun(null)).toBe("—");
  });

  it("builds pagination pages", () => {
    expect(paginationPages(3, 10)).toEqual([1, 2, 3, 4, 10]);
  });

  it("maps success rate tone", () => {
    expect(successRateTone(90)).toBe("is-good");
    expect(successRateTone(60)).toBe("is-mid");
    expect(successRateTone(20)).toBe("is-low");
  });
});
