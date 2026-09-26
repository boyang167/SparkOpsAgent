"use client";

import { useEffect, useState } from "react";
import type { ApprovalConfig, ReleasePlanVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import { defaultReleaseWindow, fromDatetimeLocalValue, toDatetimeLocalValue } from "./release-plan-utils";

export interface ReleasePlanFormValues {
  name: string;
  description: string;
  windowStart: string;
  windowEnd: string;
  managerId: string;
  approvalEnabled: boolean;
  approvalProvider: "internal" | "feishu" | "dingtalk";
  approvalWebhookUrl: string;
  scheduleExecuteTime: string;
}

interface ReleasePlanFormProps {
  open: boolean;
  title: string;
  workspaceId: string;
  busy?: boolean;
  initial?: ReleasePlanVM | null;
  onClose: () => void;
  onSubmit: (values: ReleasePlanFormValues) => void | Promise<void>;
}

function readApprovalConfig(plan?: ReleasePlanVM | null): Pick<ReleasePlanFormValues, "approvalEnabled" | "approvalProvider" | "approvalWebhookUrl"> {
  const cfg = plan?.approval_config;
  return {
    approvalEnabled: cfg?.enabled ?? plan?.approval_enabled ?? true,
    approvalProvider: (cfg?.provider as ReleasePlanFormValues["approvalProvider"]) ?? "internal",
    approvalWebhookUrl: cfg?.webhook_url ?? "",
  };
}

export function ReleasePlanForm({ open, title, workspaceId, busy, initial, onClose, onSubmit }: ReleasePlanFormProps) {
  const win = defaultReleaseWindow();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [windowStart, setWindowStart] = useState(toDatetimeLocalValue(win.start));
  const [windowEnd, setWindowEnd] = useState(toDatetimeLocalValue(win.end));
  const [scheduleExecuteTime, setScheduleExecuteTime] = useState("");
  const [managerId, setManagerId] = useState("");
  const [managerName, setManagerName] = useState("");
  const [approvalEnabled, setApprovalEnabled] = useState(true);
  const [approvalProvider, setApprovalProvider] = useState<ReleasePlanFormValues["approvalProvider"]>("internal");
  const [approvalWebhookUrl, setApprovalWebhookUrl] = useState("");

  useEffect(() => {
    if (!open) return;
    if (initial) {
      const approval = readApprovalConfig(initial);
      setName(initial.name);
      setDescription(initial.description ?? "");
      setWindowStart(toDatetimeLocalValue(initial.release_window_start));
      setWindowEnd(toDatetimeLocalValue(initial.release_window_end));
      setScheduleExecuteTime(initial.schedule_execute_time ? toDatetimeLocalValue(initial.schedule_execute_time) : "");
      setManagerId(initial.manager_id ?? "");
      setManagerName("");
      setApprovalEnabled(approval.approvalEnabled);
      setApprovalProvider(approval.approvalProvider);
      setApprovalWebhookUrl(approval.approvalWebhookUrl);
      return;
    }
    const d = defaultReleaseWindow();
    setName("");
    setDescription("");
    setWindowStart(toDatetimeLocalValue(d.start));
    setWindowEnd(toDatetimeLocalValue(d.end));
    setScheduleExecuteTime("");
    setManagerId("");
    setManagerName("");
    setApprovalEnabled(true);
    setApprovalProvider("internal");
    setApprovalWebhookUrl("");
  }, [open, initial]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      description="配置发布窗口、负责人、审批与定时执行。"
      footer={
        <>
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={busy || !name.trim()}
            onClick={() =>
              void onSubmit({
                name: name.trim(),
                description: description.trim(),
                windowStart: fromDatetimeLocalValue(windowStart),
                windowEnd: fromDatetimeLocalValue(windowEnd),
                managerId,
                approvalEnabled,
                approvalProvider,
                approvalWebhookUrl: approvalWebhookUrl.trim(),
                scheduleExecuteTime: scheduleExecuteTime ? fromDatetimeLocalValue(scheduleExecuteTime) : "",
              })
            }
          >
            {busy ? "保存中…" : "保存"}
          </button>
        </>
      }
    >
      <div className="stack-md text-sm">
        <label className="stack-xs">
          <span className="text-muted-foreground">计划名称</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="例如 v2.4 迭代发布" />
        </label>
        <label className="stack-xs">
          <span className="text-muted-foreground">描述</span>
          <textarea className="input min-h-[72px]" value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="stack-xs">
            <span className="text-muted-foreground">发布窗口开始</span>
            <input className="input" type="datetime-local" value={windowStart} onChange={(e) => setWindowStart(e.target.value)} />
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">发布窗口结束</span>
            <input className="input" type="datetime-local" value={windowEnd} onChange={(e) => setWindowEnd(e.target.value)} />
          </label>
        </div>
        <label className="stack-xs">
          <span className="text-muted-foreground">定时自动执行（可选）</span>
          <input className="input" type="datetime-local" value={scheduleExecuteTime} onChange={(e) => setScheduleExecuteTime(e.target.value)} />
          <span className="text-xs text-muted-foreground">到达该时间后自动触发首个工作流项；检查项仍需人工确认。</span>
        </label>
        <label className="stack-xs">
          <span className="text-muted-foreground">发布负责人</span>
          <WorkItemAssigneeField
            workspaceId={workspaceId}
            assigneeId={managerId || undefined}
            assigneeName={managerName}
            onSelect={(id, label) => {
              setManagerId(id ?? "");
              setManagerName(label);
            }}
          />
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={approvalEnabled} onChange={(e) => setApprovalEnabled(e.target.checked)} />
          <span>启用审批</span>
        </label>
        {approvalEnabled ? (
          <div className="stack-sm border rounded p-3">
            <label className="stack-xs">
              <span className="text-muted-foreground">审批渠道</span>
              <select className="input" value={approvalProvider} onChange={(e) => setApprovalProvider(e.target.value as ReleasePlanFormValues["approvalProvider"])}>
                <option value="internal">平台内审批</option>
                <option value="feishu">飞书 Webhook</option>
                <option value="dingtalk">钉钉 Webhook</option>
              </select>
            </label>
            {approvalProvider !== "internal" ? (
              <label className="stack-xs">
                <span className="text-muted-foreground">Webhook URL</span>
                <input
                  className="input"
                  value={approvalWebhookUrl}
                  onChange={(e) => setApprovalWebhookUrl(e.target.value)}
                  placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
                />
                <span className="text-xs text-muted-foreground">完成规划后将发送带批准/拒绝链接的卡片消息。</span>
              </label>
            ) : null}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}

export function buildApprovalConfig(values: ReleasePlanFormValues): ApprovalConfig {
  return {
    enabled: values.approvalEnabled,
    provider: values.approvalProvider,
    webhook_url: values.approvalWebhookUrl || undefined,
  };
}
