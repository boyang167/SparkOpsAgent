import { describe, expect, it } from "vitest";
import { formatDiffValue, releasePlanVersionVerbLabel } from "./release-plan-version-utils";

describe("release-plan-version-utils", () => {
  it("labels version verbs", () => {
    expect(releasePlanVersionVerbLabel("finish_planning")).toBe("完成规划");
    expect(releasePlanVersionVerbLabel("backfill")).toBe("历史回填");
  });

  it("formats diff values", () => {
    expect(formatDiffValue(true)).toBe("是");
    expect(formatDiffValue("planning")).toBe("规划中");
    expect(formatDiffValue(["u1", "u2"])).toBe("u1、u2");
  });
});
