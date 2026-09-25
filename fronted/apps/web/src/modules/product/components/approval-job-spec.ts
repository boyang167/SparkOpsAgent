import type { WorkflowNotificationVM } from "@sparkops/contracts/product";
import { toNotifyItems, type NotifyItem } from "./workflow-notify";

export type ApprovalProvider = "sparkops";

export interface ApprovalJobSpec {
  provider: ApprovalProvider;
  approverIds: string[];
  requiredApprovals: number;
  timeoutMinutes: number;
  onTimeout: "fail";
  notify: NotifyItem[];
}

function asString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
}

export function parseApprovalJobSpec(spec: Record<string, unknown> | undefined): ApprovalJobSpec {
  const approverIds = asStringArray(spec?.approver_ids);
  const required = Number(spec?.required_approvals);
  const timeout = Number(spec?.timeout_minutes);
  const notify = Array.isArray(spec?.notify) ? toNotifyItems(spec.notify as WorkflowNotificationVM[]) : [];
  const strategy = spec?.strategy && typeof spec.strategy === "object" && !Array.isArray(spec.strategy)
    ? (spec.strategy as Record<string, unknown>)
    : {};
  const strategyTimeout = Number(strategy.timeout_minutes);

  const requiredApprovals = Number.isFinite(required) && required > 0 ? Math.floor(required) : 1;

  return {
    provider: asString(spec?.provider) === "sparkops" ? "sparkops" : "sparkops",
    approverIds,
    requiredApprovals:
      approverIds.length > 0
        ? Math.min(Math.max(requiredApprovals, 1), approverIds.length)
        : Math.max(requiredApprovals, 1),
    timeoutMinutes:
      Number.isFinite(timeout) && timeout > 0
        ? timeout
        : Number.isFinite(strategyTimeout) && strategyTimeout > 0
          ? strategyTimeout
          : 60,
    onTimeout: "fail",
    notify,
  };
}

export function approvalJobSpecToRecord(spec: ApprovalJobSpec): Record<string, unknown> {
  const requiredApprovals =
    spec.approverIds.length > 0
      ? Math.min(Math.max(spec.requiredApprovals, 1), spec.approverIds.length)
      : Math.max(spec.requiredApprovals, 1);

  return {
    provider: spec.provider,
    approver_ids: spec.approverIds,
    required_approvals: requiredApprovals,
    timeout_minutes: spec.timeoutMinutes,
    strategy: { timeout_minutes: spec.timeoutMinutes, on_timeout: spec.onTimeout },
    notify: spec.notify,
  };
}

export function formatApprovalProvider(provider: ApprovalProvider): string {
  return provider === "sparkops" ? "SparkOps" : provider;
}
