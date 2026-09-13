export type CrossClusterSyncItemStatus = "pending" | "running" | "succeeded" | "failed";

export interface CrossClusterSyncItem {
  name: string;
  status: CrossClusterSyncItemStatus;
  detail?: string;
}

const STARTED_RE = /cross-cluster sync started:\s*(.+)$/i;
const SUCCEEDED_RE = /cross-cluster sync succeeded:\s*(.+)$/i;
const FAILED_RE = /cross-cluster sync failed:\s*(.+?)(?:\s*\((.*)\))?$/i;

function extractMessages(streamOutput: string): string[] {
  const messages: string[] = [];
  for (const line of streamOutput.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    try {
      const payload = JSON.parse(trimmed) as { message?: string };
      if (typeof payload.message === "string" && payload.message.trim()) {
        messages.push(payload.message.trim());
        continue;
      }
    } catch {
      // fall through — treat raw line as message
    }
    messages.push(trimmed);
  }
  return messages;
}

export function parseCrossClusterSyncProgress(streamOutput: string): CrossClusterSyncItem[] {
  const byName = new Map<string, CrossClusterSyncItem>();

  const ensure = (name: string): CrossClusterSyncItem => {
    const key = name.trim();
    const existing = byName.get(key);
    if (existing) return existing;
    const created: CrossClusterSyncItem = { name: key, status: "pending" };
    byName.set(key, created);
    return created;
  };

  for (const message of extractMessages(streamOutput)) {
    const started = message.match(STARTED_RE);
    if (started?.[1]) {
      const item = ensure(started[1]);
      if (item.status === "pending") item.status = "running";
      continue;
    }
    const succeeded = message.match(SUCCEEDED_RE);
    if (succeeded?.[1]) {
      const item = ensure(succeeded[1]);
      item.status = "succeeded";
      item.detail = undefined;
      continue;
    }
    const failed = message.match(FAILED_RE);
    if (failed?.[1]) {
      const item = ensure(failed[1]);
      item.status = "failed";
      item.detail = failed[2]?.trim() || undefined;
    }
  }

  return Array.from(byName.values());
}

export function crossClusterSyncStatusLabel(status: CrossClusterSyncItemStatus) {
  switch (status) {
    case "running":
      return "同步中";
    case "succeeded":
      return "已同步";
    case "failed":
      return "失败";
    default:
      return "等待中";
  }
}
