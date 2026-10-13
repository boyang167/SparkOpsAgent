export type NotificationTone = "success" | "danger" | "warning" | "neutral";

export interface NotificationStatus {
  label: string;
  tone: NotificationTone;
}

export type NotificationKind = "build" | "workflow" | "agent" | "other";

const STATUS_MAP: Record<string, NotificationStatus> = {
  success: { label: "成功", tone: "success" },
  succeeded: { label: "成功", tone: "success" },
  completed: { label: "完成", tone: "success" },
  failed: { label: "失败", tone: "danger" },
  failure: { label: "失败", tone: "danger" },
  error: { label: "失败", tone: "danger" },
  timeout: { label: "超时", tone: "danger" },
  cancelled: { label: "已取消", tone: "neutral" },
  canceled: { label: "已取消", tone: "neutral" },
  aborted: { label: "已中止", tone: "neutral" },
  running: { label: "运行中", tone: "warning" },
  paused: { label: "已暂停", tone: "warning" },
};

const STATUS_BODY_PATTERN = /^状态[：:]\s*(\S+)\s*$/;

/** Bodies shaped like "状态：success" become a status pill; anything else stays as text. */
export function parseNotificationBody(body: string): { status: NotificationStatus | null; text: string } {
  const trimmed = body.trim();
  const match = STATUS_BODY_PATTERN.exec(trimmed);
  if (!match) return { status: null, text: trimmed };
  const raw = match[1].toLowerCase();
  return { status: STATUS_MAP[raw] ?? { label: match[1], tone: "neutral" }, text: "" };
}

export function notificationKind(type: string): NotificationKind {
  if (type.startsWith("build.")) return "build";
  if (type.startsWith("workflow.")) return "workflow";
  if (type.startsWith("run.")) return "agent";
  return "other";
}

function startOfDay(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

export function notificationDayLabel(iso: string, now: Date = new Date()): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "更早";
  const diffDays = Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000);
  if (diffDays <= 0) return "今天";
  if (diffDays === 1) return "昨天";
  if (date.getFullYear() === now.getFullYear()) return `${date.getMonth() + 1}月${date.getDate()}日`;
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`;
}

export function notificationTimeLabel(iso: string, now: Date = new Date()): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const diffSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  if (diffSeconds < 60) return "刚刚";
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)} 分钟前`;
  const sameDay = startOfDay(now) === startOfDay(date);
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  if (sameDay) return `${Math.floor(diffSeconds / 3600)} 小时前`;
  return `${hh}:${mm}`;
}

export function groupNotificationsByDay<T extends { created_at: string }>(
  items: T[],
  now: Date = new Date(),
): Array<{ label: string; items: T[] }> {
  const groups: Array<{ label: string; items: T[] }> = [];
  for (const item of items) {
    const label = notificationDayLabel(item.created_at, now);
    const last = groups[groups.length - 1];
    if (last && last.label === label) {
      last.items.push(item);
    } else {
      groups.push({ label, items: [item] });
    }
  }
  return groups;
}
