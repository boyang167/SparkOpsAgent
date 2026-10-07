"use client";

import { useEffect, useState, type ReactNode } from "react";
import type { ProductVM } from "@sparkops/contracts/product";
import { ReleasePlanApproverField } from "./ReleasePlanApproverField";
import { ReleasePlanHookSettingsField } from "./ReleasePlanHookSettingsField";
import {
  ReleasePlanSprintAssociationField,
  ReleasePlanSprintLinkAction,
} from "./ReleasePlanSprintAssociationField";
import { RichTextField } from "./RichTextField";
import { WorkItemAssigneeField } from "./WorkItemAssigneeField";
import type { ReleasePlanFormValues, ReleasePlanSprintOption } from "./ReleasePlanForm";

function ReleasePlanCreateBlock({
  title,
  action,
  children,
  className,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`release-plan-create-block${className ? ` ${className}` : ""}`}>
      <header className="release-plan-create-block-head">
        <h3 className="release-plan-create-block-title">{title}</h3>
        {action ? <div className="release-plan-create-block-action">{action}</div> : null}
      </header>
      <div className="release-plan-create-block-body">{children}</div>
    </section>
  );
}

interface ReleasePlanFormFieldsProps {
  workspaceId: string;
  productId?: string;
  products?: ProductVM[];
  onProductChange?: (productId: string) => void;
  values: ReleasePlanFormValues;
  onChange: (patch: Partial<ReleasePlanFormValues>) => void;
  managerName?: string;
  onManagerNameChange?: (name: string) => void;
  sprintOptions?: ReleasePlanSprintOption[];
  sprintLocked?: boolean;
  onSprintOptionsChange?: (options: ReleasePlanSprintOption[]) => void;
  variant?: "default" | "sectioned" | "page";
  releaseContent?: ReactNode;
  releaseContentAction?: ReactNode;
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
  productId,
  products,
  onProductChange,
  values,
  onChange,
  managerName,
  onManagerNameChange,
  sprintOptions,
  sprintLocked,
  onSprintOptionsChange,
  variant = "default",
  releaseContent,
  releaseContentAction,
}: ReleasePlanFormFieldsProps) {
  const showSprintField = Boolean(productId) || Boolean(sprintOptions?.length) || sprintLocked;
  const lockedSprintLabel =
    sprintOptions?.find((item) => item.id === values.sprintId)?.name ?? values.sprintId;
  const [scheduleEnabled, setScheduleEnabled] = useState(Boolean(values.scheduleExecuteTime));

  useEffect(() => {
    if (values.scheduleExecuteTime) setScheduleEnabled(true);
  }, [values.scheduleExecuteTime]);

  if (variant === "page") {
    return (
      <div className="release-plan-create-form">
        <ReleasePlanCreateBlock title="基本信息">
          <div className="release-plan-create-fields release-plan-create-fields-dense">
            {products && onProductChange ? (
              <label className="form-field">
                <span className="form-label">
                  目标产品 <span className="release-plan-required">*</span>
                </span>
                <select className="input" value={productId ?? ""} onChange={(e) => onProductChange(e.target.value)}>
                  {products.length === 0 ? <option value="">暂无产品</option> : null}
                  {products.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.display_name || p.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <label className="form-field">
              <span className="form-label">
                名称 <span className="release-plan-required">*</span>
              </span>
              <input
                className="input"
                value={values.name}
                onChange={(e) => onChange({ name: e.target.value })}
                placeholder="例如 v2.4 迭代发布"
                autoFocus
              />
            </label>
            <label className="form-field">
              <span className="form-label">
                发布负责人 <span className="release-plan-required">*</span>
              </span>
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
            <label className="form-field">
              <span className="form-label">发布窗口开始</span>
              <input
                className="input"
                type="datetime-local"
                value={values.windowStart}
                onChange={(e) => onChange({ windowStart: e.target.value })}
              />
            </label>
            <label className="form-field">
              <span className="form-label">发布窗口结束</span>
              <input
                className="input"
                type="datetime-local"
                value={values.windowEnd}
                onChange={(e) => onChange({ windowEnd: e.target.value })}
              />
            </label>
            <label className="form-field">
              <span className="form-label">定时执行</span>
              <input
                className="input"
                type="datetime-local"
                value={values.scheduleExecuteTime}
                onChange={(e) => onChange({ scheduleExecuteTime: e.target.value })}
              />
            </label>
          </div>
          <p className="release-plan-create-inline-hint">定时执行到达后将自动触发首个工作流项；检查项仍需人工确认。</p>
        </ReleasePlanCreateBlock>

        <div className="release-plan-create-columns">
          <ReleasePlanCreateBlock
            title="需求关联"
            className="release-plan-create-block-fill"
            action={
            showSprintField ? (
              <ReleasePlanSprintLinkAction
                workspaceId={workspaceId}
                productId={productId}
                sprintId={values.sprintId}
                sprintOptions={sprintOptions ?? []}
                sprintLocked={sprintLocked}
                onChange={(sprintId) => onChange({ sprintId })}
                onSprintOptionsChange={onSprintOptionsChange}
              />
            ) : null
          }
        >
          {showSprintField ? (
            <ReleasePlanSprintAssociationField
              workspaceId={workspaceId}
              productId={productId}
              sprintId={values.sprintId}
              sprintOptions={sprintOptions ?? []}
              sprintLocked={sprintLocked}
              embedded
              bodyOnly
              onChange={(sprintId) => onChange({ sprintId })}
              onSprintOptionsChange={onSprintOptionsChange}
            />
          ) : null}
          <RichTextField
            className="release-plan-create-requirements"
            value={values.description}
            onChange={(html) => onChange({ description: html })}
            placeholder="请输入内容..."
          />
          </ReleasePlanCreateBlock>

          <ReleasePlanCreateBlock title="审批配置" className="release-plan-create-block-fill">
          <label className="release-plan-create-toggle">
            <input
              type="checkbox"
              checked={values.approvalEnabled}
              onChange={(e) => onChange({ approvalEnabled: e.target.checked })}
            />
            <span className="release-plan-create-toggle-body">
              <span className="release-plan-create-toggle-title">完成规划后需审批</span>
              <span className="release-plan-create-toggle-desc">规划完成后进入审批，通过后再执行发布</span>
            </span>
          </label>
          {values.approvalEnabled ? (
            <div className="release-plan-create-subfields stack-sm">
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
              {values.approvalProvider === "internal" ? (
                <div className="form-field">
                  <span className="form-label">审批人</span>
                  <ReleasePlanApproverField
                    workspaceId={workspaceId}
                    approverIds={values.approverIds}
                    onChange={(approverIds) => onChange({ approverIds })}
                  />
                </div>
              ) : null}
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
            </div>
          ) : null}
          </ReleasePlanCreateBlock>
        </div>

        <ReleasePlanCreateBlock title="发布内容" action={releaseContentAction}>
          {releaseContent ?? (
            <p className="release-plan-create-inline-hint">
              填写计划名称后，可添加检查项与工作流发布项；也可在保存草稿后继续编辑。
            </p>
          )}
        </ReleasePlanCreateBlock>

        <details className="release-plan-create-advanced">
          <summary>外部系统 Hook（可选）</summary>
          <ReleasePlanHookSettingsField
            values={values.hookSettings}
            onChange={(hookSettings) => onChange({ hookSettings })}
          />
        </details>
      </div>
    );
  }

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

        {showSprintField ? (
          <ReleasePlanSprintAssociationField
            workspaceId={workspaceId}
            productId={productId}
            sprintId={values.sprintId}
            sprintOptions={sprintOptions ?? []}
            sprintLocked={sprintLocked}
            onChange={(sprintId) => onChange({ sprintId })}
            onSprintOptionsChange={onSprintOptionsChange}
          />
        ) : null}

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
                {values.approvalProvider === "internal" ? (
                  <div className="form-field">
                    <span className="form-label">审批人</span>
                    <ReleasePlanApproverField
                      workspaceId={workspaceId}
                      approverIds={values.approverIds}
                      onChange={(approverIds) => onChange({ approverIds })}
                    />
                  </div>
                ) : null}
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

        <details className="release-plan-form-details release-plan-form-details-advanced">
          <summary className="release-plan-form-details-summary">外部系统 Hook（可选）</summary>
          <div className="mt-3 pt-3 border-t border-border">
            <ReleasePlanHookSettingsField
              values={values.hookSettings}
              onChange={(hookSettings) => onChange({ hookSettings })}
            />
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
      {showSprintField ? (
        sprintLocked ? (
          values.sprintId ? (
            <p className="text-xs text-muted-foreground">关联迭代：{lockedSprintLabel || values.sprintId}</p>
          ) : null
        ) : (
          <ReleasePlanSprintAssociationField
            workspaceId={workspaceId}
            productId={productId}
            sprintId={values.sprintId}
            sprintOptions={sprintOptions ?? []}
            onChange={(sprintId) => onChange({ sprintId })}
            onSprintOptionsChange={onSprintOptionsChange}
          />
        )
      ) : null}
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
          {values.approvalProvider === "internal" ? (
            <div className="stack-xs">
              <span className="text-muted-foreground">审批人</span>
              <ReleasePlanApproverField
                workspaceId={workspaceId}
                approverIds={values.approverIds}
                onChange={(approverIds) => onChange({ approverIds })}
              />
            </div>
          ) : null}
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
