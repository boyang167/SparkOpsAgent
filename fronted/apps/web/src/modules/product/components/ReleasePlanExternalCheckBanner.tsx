"use client";

import type { ReleasePlanVM } from "@sparkops/contracts/product";
import { releasePlanStatusLabel } from "./release-plan-utils";

const WAIT_STATUSES = new Set([
  "wait_for_finish_planning_external_check",
  "wait_for_execute_external_check",
  "wait_for_all_done_external_check",
]);

export function isReleasePlanExternalCheckWaiting(status: string): boolean {
  return WAIT_STATUSES.has(status);
}

interface ReleasePlanExternalCheckBannerProps {
  plan: ReleasePlanVM;
}

export function ReleasePlanExternalCheckBanner({ plan }: ReleasePlanExternalCheckBannerProps) {
  if (!isReleasePlanExternalCheckWaiting(plan.status)) return null;

  return (
    <div className="release-plan-external-check-banner">
      <p className="text-sm font-medium">{releasePlanStatusLabel(plan.status)}</p>
      <p className="text-xs text-muted-foreground mt-1">
        已向外部的 Webhook 发送回调请求，请在外部系统中完成校验。外部系统可通过回调 URL 继续或拒绝此计划。
        {plan.hook_state?.event ? ` 当前阶段：${plan.hook_state.event}` : ""}
      </p>
    </div>
  );
}
