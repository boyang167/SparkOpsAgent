import type { WorkflowNotificationVM } from "@sparkops/contracts/product";

export const WORKFLOW_NOTIFY_EVENTS = [
  { id: "created", label: "任务开始" },
  { id: "passed", label: "任务成功" },
  { id: "failed", label: "任务失败" },
  { id: "timeout", label: "任务超时" },
  { id: "cancelled", label: "任务取消" },
  { id: "changed", label: "状态变更" },
  { id: "rejected", label: "任务拒绝" },
  { id: "manual", label: "等待手动执行" },
  { id: "waiting", label: "等待人工确认" },
] as const;

export const WORKFLOW_NOTIFY_CHANNELS = [
  { id: "feishu_bot", label: "飞书群组通知（自定义机器人）" },
  { id: "feishu_app", label: "飞书群组通知（自建应用）" },
  { id: "feishu_user", label: "飞书成员通知" },
  { id: "dingtalk", label: "钉钉" },
  { id: "wecom", label: "企业微信" },
  { id: "teams", label: "Teams" },
  { id: "mail", label: "邮件" },
  { id: "webhook", label: "Webhook" },
] as const;

export interface NotifyConfig {
  events: string[];
  channel: string;
  webhook_url: string;
  targets: string;
}

export interface NotifyItem extends NotifyConfig {
  id: string;
  enabled: boolean;
}

export function emptyNotifyItem(): NotifyItem {
  return { id: crypto.randomUUID(), enabled: true, events: [], channel: "", webhook_url: "", targets: "" };
}
export function notifyChannelLabel(channel: string): string {
  return WORKFLOW_NOTIFY_CHANNELS.find((item) => item.id === channel)?.label ?? channel;
}

export function notifyEventLabels(events: string[]): string {
  const labels = WORKFLOW_NOTIFY_EVENTS.filter((item) => events.includes(item.id)).map((item) => item.label);
  return labels.join("、");
}

export function emptyWorkflowNotification(): WorkflowNotificationVM {
  return {
    id: crypto.randomUUID(),
    enabled: true,
    events: [],
    channel: "",
    webhook_url: "",
    targets: "",
  };
}

export function toNotifyItems(items: WorkflowNotificationVM[] | undefined): NotifyItem[] {
  return (items ?? []).map((item) => ({
    id: item.id,
    enabled: item.enabled !== false,
    events: item.events ?? [],
    channel: item.channel,
    webhook_url: item.webhook_url ?? "",
    targets: item.targets ?? "",
  }));
}

export function fromNotifyItems(items: NotifyItem[]): WorkflowNotificationVM[] {
  return items.map((item) => ({
    id: item.id,
    enabled: item.enabled,
    events: item.events,
    channel: item.channel,
    webhook_url: item.webhook_url,
    targets: item.targets,
  }));
}

export function channelNeedsWebhook(channel: string): boolean {
  return channel === "feishu_bot" || channel === "feishu_app" || channel === "dingtalk" || channel === "wecom" || channel === "teams" || channel === "webhook";
}

export function validateWorkflowNotification(item: NotifyConfig, required = false): string | null {
  const configured =
    required || item.events.length > 0 || item.channel.trim() !== "" || item.webhook_url.trim() !== "" || item.targets.trim() !== "";
  if (!configured) return null;
  if (item.events.length === 0) return "请至少选择一个通知事件";
  if (!item.channel) return "请选择通知类型";
  if (channelNeedsWebhook(item.channel)) {
    const raw = item.webhook_url.trim();
    if (!raw) return "请填写 Webhook 地址";
    try {
      const parsed = new URL(raw);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        return "Webhook 地址需以 http 或 https 开头";
      }
    } catch {
      return "Webhook 地址无效";
    }
  }
  if (item.channel === "mail" && !item.targets.trim()) return "请填写收件人";
  if (item.channel === "feishu_user" && !item.targets.trim()) return "请填写成员 ID";
  return null;
}
