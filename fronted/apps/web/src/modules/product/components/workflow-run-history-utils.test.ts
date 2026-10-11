import { describe, expect, it } from "vitest";
import {
  branchSummary,
  executorLabel,
  projectLabel,
  runStatusLabel,
  runStatusTone,
  serviceGroups,
} from "./workflow-run-history-utils";

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

  it("shortens unknown project ids instead of printing the raw uuid", () => {
    const id = "e82412bc-488f-4e6b-8624-622b9d4cf409";
    expect(projectLabel(id, {})).toBe("项目 e82412bc");
    expect(projectLabel(id, { [id]: "test-repo" })).toBe("test-repo");
  });

  it("groups build and deploy services per project", () => {
    const a = "0e03e33c-a4b9-4e74-aaf3-7b3f0398128e";
    const b = "e82412bc-488f-4e6b-8624-622b9d4cf409";
    expect(serviceGroups([b, a], [a], { [a]: "test-repo" })).toEqual([
      { id: b, label: "项目 e82412bc", known: false, kinds: ["构建"] },
      { id: a, label: "test-repo", known: true, kinds: ["构建", "部署"] },
    ]);
    expect(serviceGroups(undefined, undefined, {})).toEqual([]);
  });

  it("maps every status to a tone", () => {
    expect(runStatusTone("succeeded")).toBe("is-success");
    expect(runStatusTone("aborted")).toBe("is-neutral");
    expect(runStatusTone("unknown")).toBe("is-neutral");
  });
});
