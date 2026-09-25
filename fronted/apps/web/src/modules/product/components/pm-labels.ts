import type { PmProvider, WorkItemSyncMode } from "@sparkops/contracts/product";

export const PM_PROVIDER_LABEL: Record<PmProvider, string> = {
  jira: "Jira",
  tapd: "TAPD",
  pingcode: "PingCode",
  feishu: "飞书项目",
};

export const PM_SYNC_MODE_LABEL: Record<WorkItemSyncMode, string> = {
  link_only: "仅关联",
  bidirectional: "双向同步",
};
