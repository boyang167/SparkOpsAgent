import type { WorkflowRunVM } from "@sparkops/contracts/workflow";

export interface WorkflowStepLog {
  index: number;
  type: string;
  message: string;
}

export function parseWorkflowRunLogs(output?: string): WorkflowStepLog[] {
  if (!output?.trim()) {
    return [];
  }

  try {
    const parsed = JSON.parse(output) as { logs?: string[] };
    if (!Array.isArray(parsed.logs)) {
      return [];
    }
    return parsed.logs.map((line, i) => parseStepLine(line, i + 1));
  } catch {
    return [];
  }
}

function parseStepLine(line: string, fallbackIndex: number): WorkflowStepLog {
  const match = /^\[step (\d+)\]\[(\w+)\]\s*(.*)$/.exec(line);
  if (!match) {
    return { index: fallbackIndex, type: "log", message: line };
  }
  return {
    index: Number(match[1]),
    type: match[2],
    message: match[3] || (match[2] === "noop" ? "ok" : ""),
  };
}

export function statusBadgeClass(status: string) {
  if (status === "completed") return "badge-success";
  if (status === "failed") return "badge-warning";
  if (status === "running") return "badge-neutral";
  return "badge-neutral";
}

export function formatWorkflowTime(iso: string) {
  try {
    return new Date(iso).toLocaleString("zh-CN");
  } catch {
    return iso;
  }
}

export function summarizeWorkflowRun(run: WorkflowRunVM) {
  const logs = parseWorkflowRunLogs(run.output);
  return {
    stepCount: logs.length,
    preview: logs[0]?.message ?? run.error ?? "—",
  };
}
