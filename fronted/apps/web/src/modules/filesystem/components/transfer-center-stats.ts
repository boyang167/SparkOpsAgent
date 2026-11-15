import type { TransferSessionVM, TransferStatus as PersistedStatus } from "@sparkops/contracts/filesystem";
import type { TransferItem, TransferStatus } from "./transfer-queue";

export type TransferMetricCounts = {
  running: number;
  completed: number;
  paused: number;
  failed: number;
};

const localActive = new Set<TransferStatus>([
  "queued",
  "creating",
  "uploading",
  "verifying",
]);

const persistedRunning = new Set<PersistedStatus>([
  "INITIATED",
  "UPLOADING",
  "VERIFYING",
  "SCANNING",
  "COMMITTING",
]);

const persistedFailed = new Set<PersistedStatus>(["FAILED", "REJECTED", "EXPIRED"]);

export function computeTransferMetrics(
  localItems: readonly TransferItem[],
  persisted: readonly TransferSessionVM[],
): TransferMetricCounts {
  let running = 0;
  let completed = 0;
  let paused = 0;
  let failed = 0;

  for (const item of localItems) {
    if (localActive.has(item.status)) running += 1;
    else if (item.status === "completed") completed += 1;
    else if (item.status === "failed") failed += 1;
  }

  for (const session of persisted) {
    if (persistedRunning.has(session.status)) running += 1;
    else if (session.status === "COMPLETED") completed += 1;
    else if (session.status === "PAUSED") paused += 1;
    else if (persistedFailed.has(session.status)) failed += 1;
  }

  return { running, completed, paused, failed };
}

export function transferDisplayId(sessionId: string) {
  const compact = sessionId.replace(/-/g, "").slice(0, 8).toUpperCase();
  return `TRF-${compact}`;
}

export function formatByteCount(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}