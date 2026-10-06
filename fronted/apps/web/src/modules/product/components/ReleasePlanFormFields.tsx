"use client";

import { useEffect, useState, type ReactNode } from "react";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import type { ReleasePlanFormValues } from "./ReleasePlanForm";

interface ReleasePlanFormFieldsProps {
  workspaceId: string;
  values: ReleasePlanFormValues;
  onChange: (patch: Partial<ReleasePlanFormValues>) => void;
  managerName?: string;
  onManagerNameChange?: (name: string) => void;
  variant?: "default" | "sectioned";
}

function FormSection({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="release-plan-form-section">
      <div className="release-plan-form-section-head">
        <h3 className="release-plan-form-section-title">{title}</h3>
        {hint ? <p className="release-plan-form-section-hint">{hint}</p> : null}
      </div>
      <div className="release-plan-form-section-body">{children}</div>
    </section>
  );
}

export function ReleasePlanFormFields({
  workspaceId,
  values,
  onChange,
  managerName,
  onManagerNameChange,
  variant = "default",
}: ReleasePlanFormFieldsProps) {
  const [scheduleEnabled, setScheduleEnabled] = useState(Boolean(values.scheduleExecuteTime));

  useEffect(() => {
    if (values.scheduleExecuteTime) setScheduleEnabled(true);
  }, [values.scheduleExecuteTime]);

  if (variant === "sectioned") {
    return (
      <div className="release-plan-form stack-md">
        <FormSection title="基本信息" hint="创建后可继续添加发布项与检查清单">
          <label className="form-field">
            <span className="form-label">计划名称 *</span>
            <input
              className="input"
              value={values.name}
              onChange={(e) => onChange({ name: e.target.value })}
              placeholder="例如 v2.4 迭代发布"
              autoFocus
            />
          </label>
          <details className="release-plan-form-details">
            <summary className="release-plan-form-details-summary">添加描述（可选）</summary>
            <label className="form-field mt-2">
              <textarea
                className="input product-create-textarea min-h-[4rem]"
                value={values.description}
                onChange={(e) => onChange({ description: e.target.value })}
                placeholder="补充发布背景、风险或回滚策略"
              />
            </label>
          </details>
        </FormSection>

        <FormSection title="发布窗口">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="form-field">
              <span className="form-label">开始时间</span>
              <input
                className="input"
                type="datetime-local"
                value={values.windowStart}
                onChange={(e) => onChange({ windowStart: e.target.value })}
              />
            </label>
            <label className="form-field">
              <span className="form-label">结束时间</span>
              <input
                className="input"
                type="datetime-local"
                value={values.windowEnd}
                onChange={(e) => onChange({ windowEnd: e.target.value })}
              />
            </label>
          </div>
        </FormSection>

        <FormSection title="执行与负责人">
          <label className="form-field">
            <span className="form-label">发布负责人</span>
            <WorkItemAssigneeField
              workspaceId={workspaceId}
              assigneeId={values.managerId || undefined}
              assigneeName={managerName}
              onSelect={(id, label) => {
                onChange({ managerId: id ?? "" });
                onManagerNameChange?.(label);
              }}
            />
          </label>
          <label className="flex items-center gap-2 py-1">
            <input
              type="checkbox"
              checked={scheduleEnabled}
              onChange={(e) => {
                setScheduleEnabled(e.target.checked);
                if (!e.target.checked) onChange({ scheduleExecuteTime: "" });
              }}
            />
            <span className="text-sm">到达指定时间自动触发首个工作流项</span>
          </label>
          {scheduleEnabled ? (
            <label className="form-field">
              <span className="form-label">定时执行时间</span>
              <input
                className="input"
                type="datetime-local"
                value={values.scheduleExecuteTime}
                onChange={(e) => onChange({ scheduleExecuteTime: e.target.value })}
              />
              <p className="field-hint text-xs text-muted-foreground">检查项仍需人工确认，不受定时影响。</p>
            </label>
          ) : null}
        </FormSection>

        <details className="release-plan-form-details release-plan-form-details-advanced">
          <summary className="release-plan-form-details-summary">审批与通知（可选）</summary>
          <div className="stack-sm mt-3 pt-3 border-t border-border">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={values.approvalEnabled}
                onChange={(e) => onChange({ approvalEnabled: e.target.checked })}
              />
              <span className="text-sm">完成规划后需审批</span>
            </label>
            {values.approvalEnabled ? (
              <>
                <label className="form-field">
                  <span className="form-label">审批渠道</span>
                  <select
                    className="input"
                    value={values.approvalProvider}
                    onChange={(e) =>
                      onChange({ approvalProvider: e.target.value as ReleasePlanFormValues["approvalProvider"] })
                    }
                  >
                    <option value="internal">平台内审批</option>
                    <option value="feishu">飞书 Webhook</option>
                    <option value="dingtalk">钉钉 Webhook</option>
                  </select>
                </label>
                {values.approvalProvider !== "internal" ? (
                  <label className="form-field">
                    <span className="form-label">Webhook URL</span>
                    <input
                      className="input"
                      value={values.approvalWebhookUrl}
                      onChange={(e) => onChange({ approvalWebhookUrl: e.target.value })}
                      placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
                    />
                  </label>
                ) : null}
              </>
            ) : null}
          </div>
        </details>
      </div>
    );
  }

  return (
    <div className="stack-md text-sm">
      <label className="stack-xs">
        <span className="text-muted-foreground">计划名称</span>
        <input
          className="input"
          value={values.name}
          onChange={(e) => onChange({ name: e.target.value })}
          placeholder="例如 v2.4 迭代发布"
        />
      </label>
      <label className="stack-xs">
        <span className="text-muted-foreground">描述</span>
        <textarea
          className="input min-h-[72px]"
          value={values.description}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </label>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="stack-xs">
          <span className="text-muted-foreground">发布窗口开始</span>
          <input
            className="input"
            type="datetime-local"
            value={values.windowStart}
            onChange={(e) => onChange({ windowStart: e.target.value })}
          />
        </label>
        <label className="stack-xs">
          <span className="text-muted-foreground">发布窗口结束</span>
          <input
            className="input"
            type="datetime-local"
            value={values.windowEnd}
            onChange={(e) => onChange({ windowEnd: e.target.value })}
          />
        </label>
      </div>
      <label className="stack-xs">
        <span className="text-muted-foreground">定时自动执行（可选）</span>
        <input
          className="input"
          type="datetime-local"
          value={values.scheduleExecuteTime}
          onChange={(e) => onChange({ scheduleExecuteTime: e.target.value })}
        />
        <span className="text-xs text-muted-foreground">到达该时间后自动触发首个工作流项；检查项仍需人工确认。</span>
      </label>
      <label className="stack-xs">
        <span className="text-muted-foreground">发布负责人</span>
        <WorkItemAssigneeField
          workspaceId={workspaceId}
          assigneeId={values.managerId || undefined}
          assigneeName={managerName}
          onSelect={(id, label) => {
            onChange({ managerId: id ?? "" });
            onManagerNameChange?.(label);
          }}
        />
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={values.approvalEnabled}
          onChange={(e) => onChange({ approvalEnabled: e.target.checked })}
        />
        <span>启用审批</span>
      </label>
      {values.approvalEnabled ? (
        <div className="stack-sm border rounded p-3">
          <label className="stack-xs">
            <span className="text-muted-foreground">审批渠道</span>
            <select
              className="input"
              value={values.approvalProvider}
              onChange={(e) =>
                onChange({ approvalProvider: e.target.value as ReleasePlanFormValues["approvalProvider"] })
              }
            >
              <option value="internal">平台内审批</option>
              <option value="feishu">飞书 Webhook</option>
              <option value="dingtalk">钉钉 Webhook</option>
            </select>
          </label>
          {values.approvalProvider !== "internal" ? (
            <label className="stack-xs">
              <span className="text-muted-foreground">Webhook URL</span>
              <input
                className="input"
                value={values.approvalWebhookUrl}
                onChange={(e) => onChange({ approvalWebhookUrl: e.target.value })}
                placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
              />
              <span className="text-xs text-muted-foreground">完成规划后将发送带批准/拒绝链接的卡片消息。</span>
            </label>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
