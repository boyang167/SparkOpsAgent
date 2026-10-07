import { describe, expect, it } from "vitest";
import { formatAuditDetail, releasePlanAuditActionLabel } from "./release-plan-utils";

describe("releasePlanAuditActionLabel", () => {
  it("maps known actions to Chinese labels", () => {
    expect(releasePlanAuditActionLabel("finish_planning")).toBe("完成规划");
    expect(releasePlanAuditActionLabel("unknown_action")).toBe("unknown_action");
  });
});

describe("formatAuditDetail", () => {
  it("formats reject reason and job metadata", () => {
    expect(
      formatAuditDetail({
        reason: "窗口冲突",
        job_id: "01234567-abcd-efgh",
        workflow_run_id: "89abcdef-0123-4567",
      }),
    ).toBe("原因：窗口冲突 · 发布项：01234567… · Run：89abcdef…");
  });

  it("returns null for empty detail", () => {
    expect(formatAuditDetail(null)).toBeNull();
    expect(formatAuditDetail({})).toBeNull();
  });
});
