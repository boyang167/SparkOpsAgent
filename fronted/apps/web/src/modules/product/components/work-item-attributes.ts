import type { WorkItemPriority, WorkItemType } from "@sparkops/contracts/product";

export const DEFAULT_WORK_ITEM_TYPE: WorkItemType = "story";
export const DEFAULT_WORK_ITEM_PRIORITY: WorkItemPriority = "medium";

export const WORK_ITEM_TYPE_OPTIONS: ReadonlyArray<{ value: WorkItemType; label: string }> = [
  { value: "story", label: "需求" },
  { value: "task", label: "任务" },
  { value: "bug", label: "缺陷" },
  { value: "epic", label: "史诗" },
  { value: "release", label: "发布" },
  { value: "custom", label: "自定义" },
];

export const WORK_ITEM_PRIORITY_OPTIONS: ReadonlyArray<{ value: WorkItemPriority; label: string }> = [
  { value: "critical", label: "紧急" },
  { value: "high", label: "高" },
  { value: "medium", label: "中" },
  { value: "low", label: "低" },
];

export function workItemTypeLabel(type: string): string {
  return WORK_ITEM_TYPE_OPTIONS.find((item) => item.value === type)?.label ?? type;
}

export function workItemPriorityLabel(priority: string): string {
  return WORK_ITEM_PRIORITY_OPTIONS.find((item) => item.value === priority)?.label ?? priority;
}

export interface WorkItemMetaPart {
  key: "type" | "priority";
  label: string;
  tone?: string;
}

/** Card meta only surfaces values that differ from the creation defaults. */
export function workItemMetaParts(item: { type: string; priority: string }): WorkItemMetaPart[] {
  const parts: WorkItemMetaPart[] = [];
  if (item.type && item.type !== DEFAULT_WORK_ITEM_TYPE) {
    parts.push({ key: "type", label: workItemTypeLabel(item.type), tone: item.type });
  }
  if (item.priority && item.priority !== DEFAULT_WORK_ITEM_PRIORITY) {
    parts.push({ key: "priority", label: `${workItemPriorityLabel(item.priority)}优先级`, tone: item.priority });
  }
  return parts;
}
