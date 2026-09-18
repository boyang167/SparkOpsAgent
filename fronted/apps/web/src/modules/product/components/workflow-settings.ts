import type { WorkflowTriggerVM, WorkflowVariableVM } from "@sparkops/contracts/product";

export const SYSTEM_WORKFLOW_VARIABLES = [
  { key: "WORKFLOW_NAME", label: "工作流名称" },
  { key: "WORKFLOW_KEY", label: "工作流标识" },
  { key: "PRODUCT_ID", label: "产品 ID" },
  { key: "RUN_ID", label: "本次运行 ID" },
  { key: "GIT_BRANCH", label: "Git 分支" },
  { key: "GIT_TAG", label: "Git Tag" },
  { key: "GIT_COMMIT", label: "Git Commit" },
  { key: "GIT_EVENT", label: "Git 事件" },
] as const;

export const GIT_TRIGGER_EVENTS = [
  { id: "push", label: "Push" },
  { id: "tag", label: "Tag" },
  { id: "merge_request", label: "Merge Request" },
] as const;

export function newSettingId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `id-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function newHookToken() {
  return newSettingId().replace(/-/g, "");
}

export function emptyVariable(): WorkflowVariableVM {
  return { id: newSettingId(), key: "", value: "", description: "", runtime_input: false, secret: false, vault_path: "" };
}

export function emptyTrigger(type: WorkflowTriggerVM["type"]): WorkflowTriggerVM {
  return {
    id: newSettingId(),
    type,
    enabled: true,
    project_id: "",
    events: type === "git" ? ["push"] : [],
    branch_filter: "",
    token: type === "cron" ? "" : newHookToken(),
    schedule: type === "cron" ? "0 2 * * *" : "",
  };
}

export function variableKeyError(key: string) {
  const trimmed = key.trim();
  if (!trimmed) return "请填写变量名";
  if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(trimmed)) return "变量名需以字母开头，只含字母、数字和下划线";
  return null;
}
