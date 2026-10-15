import { describe, expect, it } from "vitest";
import type { ReleasePlanJobVM } from "@sparkops/contracts/product";
import {
  canOverrideInJob,
  formatMissingInputs,
  jobInputStates,
  planInputRows,
  planMissingInputs,
  runModalExtraInputs,
} from "./release-plan-runtime-inputs";

const ref = (stage_index: number, job_name: string, field = "version_tag") => ({
  stage_index,
  stage_name: `阶段${stage_index + 1}`,
  job_name,
  job_type: "sparkops-create-version",
  field,
});

function workflowJob(partial: Partial<ReleasePlanJobVM>): ReleasePlanJobVM {
  return { id: "j1", name: "dev", type: "workflow", status: "todo", ...partial };
}

describe("release plan runtime inputs", () => {
  it("treats reserved and underscore keys as plan-only", () => {
    expect(canOverrideInJob("version")).toBe(true);
    expect(canOverrideInJob("branch")).toBe(false);
    expect(canOverrideInJob("environment_id")).toBe(false);
    expect(canOverrideInJob("_project_ids")).toBe(false);
  });

  it("unions inputs across workflow jobs with usage labels", () => {
    const rows = planInputRows([
      workflowJob({ id: "a", name: "dev", runtime_inputs: [{ key: "version", label: "version", declared: false, refs: [ref(2, "archive")] }] }),
      workflowJob({ id: "b", name: "prod", runtime_inputs: [{ key: "version", label: "版本", declared: true, secret: true }] }),
      workflowJob({ id: "c", name: "skip", status: "skipped", runtime_inputs: [{ key: "ticket", label: "ticket", declared: false }] }),
    ]);
    expect(rows).toEqual([
      { key: "version", label: "version", secret: true, usedBy: ["dev · 阶段3 · archive", "prod · 运行时变量"] },
    ]);
  });

  it("resolves job override, plan inheritance and missing values", () => {
    const job = workflowJob({
      workflow_ref: { workflow_id: "wf", parameters: { version: " 2.0.0 ", branch: "main" } },
      runtime_inputs: [
        { key: "version", label: "version", declared: false },
        { key: "ticket", label: "工单", declared: true },
        { key: "branch", label: "branch", declared: false },
        { key: "env", label: "env", declared: false },
      ],
    });
    const states = jobInputStates(job, { ticket: "OPS-1", env: " " });
    expect(states.map((s) => [s.key, s.source, s.value, s.overridable])).toEqual([
      ["version", "job", "2.0.0", true],
      ["ticket", "plan", "OPS-1", true],
      ["branch", "missing", "", false],
      ["env", "missing", "", true],
    ]);
  });

  it("lists missing inputs per pending workflow job", () => {
    const plan = {
      parameters: { version: "1.0.0" },
      jobs: [
        workflowJob({ id: "a", name: "dev", runtime_inputs: [{ key: "version", label: "v", declared: false }] }),
        workflowJob({ id: "b", name: "prod", runtime_inputs: [{ key: "ticket", label: "t", declared: false }, { key: "env", label: "e", declared: false }] }),
        { id: "c", name: "检查", type: "text" as const, status: "todo" },
      ],
    };
    const missing = planMissingInputs(plan);
    expect(missing).toEqual([{ jobId: "b", jobName: "prod", keys: ["ticket", "env"] }]);
    expect(formatMissingInputs(missing)).toBe("请先填写运行参数：「prod」ticket、env");
  });

  it("keeps only undeclared inputs that the version field does not cover", () => {
    const extra = runModalExtraInputs([
      { key: "ENV", label: "ENV", declared: true },
      { key: "version", label: "version", declared: false, refs: [ref(2, "archive")] },
      { key: "ticket", label: "ticket", declared: false, refs: [ref(2, "archive"), ref(0, "build", "branch")] },
      { key: "note", label: "note", declared: false },
    ]);
    expect(extra.map((item) => item.key)).toEqual(["ticket", "note"]);
  });
});
