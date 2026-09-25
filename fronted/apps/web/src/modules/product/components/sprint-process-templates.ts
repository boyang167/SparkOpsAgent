import type { SprintStage } from "@sparkops/contracts/product";

const STAGE_COLORS = ["#3b82f6", "#8b5cf6", "#f59e0b", "#10b981", "#64748b", "#06b6d4", "#ec4899", "#ef4444"];

function newStage(partial: Pick<SprintStage, "key" | "name"> & { color?: string }): SprintStage {
  return {
    id: crypto.randomUUID(),
    key: partial.key,
    name: partial.name,
    sort_order: 0,
    color: partial.color ?? STAGE_COLORS[0],
    workflows: [],
  };
}

export const SPRINT_PROCESS_TEMPLATES: Array<{
  id: string;
  label: string;
  description: string;
  stages: SprintStage[];
}> = [
  {
    id: "agile-4",
    label: "标准敏捷（4 列）",
    description: "开发 → 测试 → 预发 → 发布",
    stages: [
      newStage({ key: "dev", name: "开发", color: "#3b82f6" }),
      newStage({ key: "test", name: "测试", color: "#8b5cf6" }),
      newStage({ key: "uat", name: "预发", color: "#f59e0b" }),
      newStage({ key: "release", name: "发布", color: "#10b981" }),
    ],
  },
  {
    id: "lean-3",
    label: "精简 Kanban（3 列）",
    description: "待办 → 进行中 → 完成",
    stages: [
      newStage({ key: "todo", name: "待办", color: "#64748b" }),
      newStage({ key: "doing", name: "进行中", color: "#3b82f6" }),
      newStage({ key: "done", name: "完成", color: "#10b981" }),
    ],
  },
  {
    id: "devops-6",
    label: "DevOps 全流程（6 列）",
    description: "需求 → 开发 → CI → 测试 → 预发 → 生产",
    stages: [
      newStage({ key: "backlog", name: "需求", color: "#64748b" }),
      newStage({ key: "dev", name: "开发", color: "#3b82f6" }),
      newStage({ key: "ci", name: "CI", color: "#06b6d4" }),
      newStage({ key: "test", name: "测试", color: "#8b5cf6" }),
      newStage({ key: "uat", name: "预发", color: "#f59e0b" }),
      newStage({ key: "prod", name: "生产", color: "#10b981" }),
    ],
  },
];

export function withSortOrder(stages: SprintStage[]): SprintStage[] {
  return stages.map((stage, index) => ({ ...stage, sort_order: index }));
}

const STAGE_KEY_PATTERN = /^[a-z][a-z0-9_]{0,63}$/;

export function isValidStageKey(key: string): boolean {
  return STAGE_KEY_PATTERN.test(key.trim().toLowerCase());
}

export function slugifyStageKey(name: string, index: number): string {
  const slug = name
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "_")
    .replace(/[^a-z0-9_]/g, "")
    .replace(/^[0-9]+/, "");
  if (slug && STAGE_KEY_PATTERN.test(slug)) {
    return slug.slice(0, 64);
  }
  return `stage_${index + 1}`;
}

export function normalizeStageForSave(stage: SprintStage, index: number): SprintStage {
  const name = stage.name.trim();
  const key = isValidStageKey(stage.key) ? stage.key.trim().toLowerCase() : slugifyStageKey(name, index);
  return {
    ...stage,
    name,
    key,
    sort_order: index,
    wip_limit: stage.wip_limit && stage.wip_limit > 0 ? stage.wip_limit : undefined,
  };
}

export function nextStageColor(index: number): string {
  return STAGE_COLORS[index % STAGE_COLORS.length];
}
