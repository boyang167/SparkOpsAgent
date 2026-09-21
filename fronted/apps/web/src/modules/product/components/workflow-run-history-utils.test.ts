import { describe, expect, it } from "vitest";
import { branchSummary, executorLabel, runStatusLabel } from "./workflow-run-history-utils";

describe("workflow-run-history-utils", () => {
  it("maps run status labels", () => {
    expect(runStatusLabel("success")).toBe("成功");
    expect(runStatusLabel("failed")).toBe("失败");
  });

  it("resolves executor username from member email", () => {
    expect(
      executorLabel(
        { id: "r1", status: "success", current_stage: 0, executor_id: "m1" },
        { m1: { email: "m168168@example.com", display_name: "测试" } },
      ),
    ).toBe("m168168");
  });

  it("falls back to trigger source when executor is missing", () => {
    expect(
      executorLabel({ id: "r1", status: "success", current_stage: 0, trigger_source: "git" }, {}),
    ).toBe("Git");
  });

  it("summarizes branch info", () => {
    expect(
      branchSummary({
        id: "r1",
        status: "success",
        current_stage: 0,
        branches: [{ branch: "main" }, { branch: "develop" }],
      }),
    ).toBe("main / develop");
  });
});
