import { describe, expect, it } from "vitest";
import type { EditorStage } from "./workflow-editor-utils";
import { applyWorkflowDrop, moveJob, reorderStages } from "./workflow-editor-dnd";

const stages: EditorStage[] = [
  { name: "build", parallel: false, exec_mode: "auto", jobs: [{ name: "build-1", type: "sparkops-build", spec: {} }] },
  { name: "deploy", parallel: false, exec_mode: "auto", jobs: [{ name: "deploy-1", type: "sparkops-deploy", spec: {} }] },
  { name: "approval", parallel: false, exec_mode: "auto", jobs: [{ name: "approval-1", type: "sparkops-approval", spec: {} }] },
];

describe("workflow editor dnd", () => {
  it("reorders stages", () => {
    const next = reorderStages(stages, 0, 2);
    expect(next.map((stage) => stage.name)).toEqual(["deploy", "approval", "build"]);
  });

  it("moves jobs across stages", () => {
    const next = moveJob(stages, { stageIndex: 0, jobIndex: 0 }, { stageIndex: 2, jobIndex: 1 });
    expect(next[0]?.jobs).toHaveLength(0);
    expect(next[2]?.jobs.map((job) => job.name)).toEqual(["approval-1", "build-1"]);
  });

  it("reorders jobs within a stage", () => {
    const local: EditorStage[] = [
      {
        name: "build",
        parallel: false,
        exec_mode: "auto",
        jobs: [
          { name: "a", type: "sparkops-build", spec: {} },
          { name: "b", type: "sparkops-build", spec: {} },
          { name: "c", type: "sparkops-build", spec: {} },
        ],
      },
    ];
    const next = moveJob(local, { stageIndex: 0, jobIndex: 0 }, { stageIndex: 0, jobIndex: 2 });
    expect(next[0]?.jobs.map((job) => job.name)).toEqual(["b", "a", "c"]);
  });

  it("applies stage drop payload", () => {
    const next = applyWorkflowDrop(stages, { kind: "stage", stageIndex: 2 }, { kind: "stage", stageIndex: 0 });
    expect(next.map((stage) => stage.name)).toEqual(["approval", "build", "deploy"]);
  });
});
