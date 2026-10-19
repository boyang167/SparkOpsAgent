import { describe, expect, it } from "vitest";
import {
  describeJobSnapshot,
  describeWorkflowRefDiff,
  diffChangeSummary,
  formatDiffValue,
  releasePlanVersionVerbLabel,
} from "./release-plan-version-utils";

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

  it("summarizes workflow snapshot instead of dumping json", () => {
    const value = formatDiffValue({
      mode: "snapshot",
      workflow_id: "wf-1",
      workflow_snapshot: {
        name: "dev",
        stages: [{ jobs: [{ name: "build" }, { name: "deploy" }] }, { jobs: [{ name: "sync" }] }],
      },
    });
    expect(value).toContain("dev");
    expect(value).toContain("2 个阶段");
    expect(value).toContain("3 个任务");
    expect(value).not.toContain("\"stages\"");
  });

  it("describes job snapshot fields", () => {
    const sections = describeJobSnapshot({
      name: "dev",
      type: "workflow",
      sort_order: 0,
      workflow_ref: {
        mode: "reference",
        workflow_id: "wf-123",
        parameters: { version: "1.0.0", branch: "main" },
      },
    });
    expect(sections[0]?.fields.map((field) => field.label)).toEqual(["名称", "类型", "排序"]);
    expect(sections[1]?.title).toBe("工作流配置");
    expect(sections[1]?.fields.some((field) => field.label === "版本号" && field.value === "1.0.0")).toBe(true);
  });

  it("describes workflow ref diff as readable rows", () => {
    const fields = describeWorkflowRefDiff(
      { mode: "reference", workflow_id: "wf-1", parameters: { branch: "main" } },
      { mode: "reference", workflow_id: "wf-1", parameters: { branch: "release" } },
    );
    expect(fields).toEqual([{ label: "分支", value: "main → release", mono: false }]);
  });

  it("hides json summary for structured workflow changes", () => {
    const summary = diffChangeSummary({
      label: "工作流配置",
      change_type: "field_changed",
      before: { mode: "reference", workflow_id: "wf-1" },
      after: { mode: "snapshot", workflow_id: "wf-1", workflow_snapshot: { name: "dev", stages: [] } },
    });
    expect(summary).toBeNull();
  });
});
