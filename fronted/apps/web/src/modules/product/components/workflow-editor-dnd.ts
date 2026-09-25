import type { EditorJob, EditorStage } from "./workflow-editor-utils";

export type WorkflowDragPayload =
  | { kind: "stage"; stageIndex: number }
  | { kind: "job"; stageIndex: number; jobIndex: number };

export type WorkflowDropTarget =
  | { kind: "stage"; stageIndex: number }
  | { kind: "job"; stageIndex: number; jobIndex: number };

export const WORKFLOW_DRAG_MIME = "application/x-sparkops-workflow-dnd";

export function encodeWorkflowDragPayload(payload: WorkflowDragPayload): string {
  return JSON.stringify(payload);
}

export function decodeWorkflowDragPayload(raw: string): WorkflowDragPayload | null {
  try {
    const payload = JSON.parse(raw) as WorkflowDragPayload;
    if (payload?.kind === "stage" && Number.isInteger(payload.stageIndex)) return payload;
    if (
      payload?.kind === "job" &&
      Number.isInteger(payload.stageIndex) &&
      Number.isInteger(payload.jobIndex)
    ) {
      return payload;
    }
    return null;
  } catch {
    return null;
  }
}

export function readWorkflowDragPayload(dataTransfer: DataTransfer | null): WorkflowDragPayload | null {
  if (!dataTransfer) return null;
  const raw = dataTransfer.getData(WORKFLOW_DRAG_MIME);
  return raw ? decodeWorkflowDragPayload(raw) : null;
}

export function reorderStages(stages: EditorStage[], fromIndex: number, toIndex: number): EditorStage[] {
  if (
    fromIndex === toIndex ||
    fromIndex < 0 ||
    toIndex < 0 ||
    fromIndex >= stages.length ||
    toIndex >= stages.length
  ) {
    return stages;
  }
  const next = [...stages];
  const [item] = next.splice(fromIndex, 1);
  if (!item) return stages;
  next.splice(toIndex, 0, item);
  return next;
}

export function moveJob(
  stages: EditorStage[],
  from: { stageIndex: number; jobIndex: number },
  to: { stageIndex: number; jobIndex: number },
): EditorStage[] {
  const sourceJob = stages[from.stageIndex]?.jobs[from.jobIndex];
  if (!sourceJob) return stages;
  if (
    from.stageIndex === to.stageIndex &&
    (from.jobIndex === to.jobIndex || from.jobIndex === to.jobIndex - 1)
  ) {
    return stages;
  }

  const next = stages.map((stage) => ({ ...stage, jobs: [...stage.jobs] }));
  next[from.stageIndex]?.jobs.splice(from.jobIndex, 1);

  let insertAt = to.jobIndex;
  if (from.stageIndex === to.stageIndex && from.jobIndex < to.jobIndex) {
    insertAt -= 1;
  }
  insertAt = Math.max(0, Math.min(insertAt, next[to.stageIndex]?.jobs.length ?? 0));
  next[to.stageIndex]?.jobs.splice(insertAt, 0, sourceJob);
  return next;
}

export function appendJob(
  stages: EditorStage[],
  from: { stageIndex: number; jobIndex: number },
  toStageIndex: number,
): EditorStage[] {
  const targetLength = stages[toStageIndex]?.jobs.length ?? 0;
  return moveJob(stages, from, { stageIndex: toStageIndex, jobIndex: targetLength });
}

export function applyWorkflowDrop(
  stages: EditorStage[],
  payload: WorkflowDragPayload,
  target: WorkflowDropTarget,
): EditorStage[] {
  if (payload.kind === "stage" && target.kind === "stage") {
    return reorderStages(stages, payload.stageIndex, target.stageIndex);
  }
  if (payload.kind === "job" && target.kind === "job") {
    return moveJob(
      stages,
      { stageIndex: payload.stageIndex, jobIndex: payload.jobIndex },
      { stageIndex: target.stageIndex, jobIndex: target.jobIndex },
    );
  }
  if (payload.kind === "job" && target.kind === "stage") {
    return appendJob(stages, { stageIndex: payload.stageIndex, jobIndex: payload.jobIndex }, target.stageIndex);
  }
  return stages;
}

export function jobDragEnabled(job: EditorJob): boolean {
  return Boolean(job.name.trim());
}
