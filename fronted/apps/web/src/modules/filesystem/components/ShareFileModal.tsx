"use client";

import { createElement, useEffect, useMemo, useRef, useState } from "react";
import type { ReactElement } from "react";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import type {
  FileNodeVM,
  GovernedPermission,
  GovernedShareSubmissionVM,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  createGovernedGrant,
  createShareRequest,
} from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  allowedActions,
  createInitialShareForm,
  impactSummary,
  submitLabel,
} from "./share-file-form";
import type { ShareAudienceOption } from "./share-file-form";

export type ShareFileModalProps = {
  open: boolean;
  node: FileNodeVM;
  organizationId: string;
  organizationLabel: string;
  workspaceId: string;
  workspaceLabel: string;
  members: WorkspaceMemberVM[];
  memberDirectoryAvailable?: boolean;
  effectiveActions: GovernedPermission[];
  onClose: () => void;
};

type SuccessState = "ACTIVE" | "PENDING_APPROVAL";

function idempotencyKey() {
  return `share-${globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`}`;
}

function memberLabel(member: WorkspaceMemberVM) {
  return member.display_name.trim() || member.email;
}

function errorMessage(error: unknown) {
  if (!(error instanceof BffError)) return "共享失败，请检查网络后重试。";
  if (error.status === 403) return "无权共享此内容，请联系文件管理员。";
  if (error.status === 409) return "共享状态已变化，请刷新后重试。";
  if (error.status === 422) return "当前受众或权限不符合治理策略，请调整后重试。";
  return "共享失败，请稍后重试。";
}

function isPending(result: GovernedShareSubmissionVM) {
  return !("grants" in result) || result.request.status === "PENDING";
}

export function ShareFileModal({
  open,
  node,
  organizationId,
  organizationLabel,
  workspaceId,
  workspaceLabel,
  members,
  memberDirectoryAvailable = true,
  effectiveActions,
  onClose,
}: ShareFileModalProps): ReactElement {
  const [step, setStep] = useState<1 | 2>(1);
  const [selectedKey, setSelectedKey] = useState("");
  const [form, setForm] = useState(() => createInitialShareForm(node.type));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<SuccessState | null>(null);
  const keyRef = useRef(idempotencyKey());
  const classification = node.classification ?? "RESTRICTED";

  const audiences = useMemo<ShareAudienceOption[]>(() => [
    {
      principalType: "ORGANIZATION",
      principalId: organizationId,
      label: organizationLabel,
      approvalRequired: true,
      disabledReason:
        classification === "RESTRICTED"
          ? "受限内容不能共享给整个组织"
          : "组织成员目录不可用，无法提供准确影响人数",
    },
    {
      principalType: "WORKSPACE",
      principalId: workspaceId,
      label: workspaceLabel,
      memberCount: members.length,
      approvalRequired: true,
      disabledReason: memberDirectoryAvailable
        ? undefined
        : "工作区成员目录不可用，无法提供准确影响人数",
    },
    ...members.map((member) => ({
      principalType: "USER" as const,
      principalId: member.member_id,
      label: memberLabel(member),
      memberCount: 1,
      approvalRequired: false,
    })),
  ], [
    classification,
    members,
    memberDirectoryAvailable,
    organizationId,
    organizationLabel,
    workspaceId,
    workspaceLabel,
  ]);
  const selected = audiences.find(
    (option) => `${option.principalType}:${option.principalId}` === selectedKey,
  );
  const actionOptions = allowedActions(selected?.principalType ?? "USER");

  useEffect(() => {
    if (!open) return;
    setStep(1);
    setSelectedKey("");
    setForm(createInitialShareForm(node.type));
    setSubmitting(false);
    setError(null);
    setSuccess(null);
    keyRef.current = idempotencyKey();
  }, [node.id, node.type, open]);

  function toggleAction(action: GovernedPermission) {
    setForm((current) => ({
      ...current,
      actions: current.actions.includes(action)
        ? current.actions.filter((item) => item !== action)
        : [...current.actions, action],
    }));
  }

  async function submit() {
    if (!selected || selected.disabledReason || form.actions.length === 0) return;
    if (!effectiveActions.includes("SHARE")) {
      setError("无权共享此内容。");
      return;
    }
    setSubmitting(true);
    setError(null);
    const input = {
      principal_type: selected.principalType,
      principal_ids: [selected.principalId],
      actions: form.actions,
      effect: form.effect,
      inherit_to_children: node.type === "DIRECTORY" ? form.inheritToChildren : false,
      expires_at: form.expiresAt ? new Date(form.expiresAt).toISOString() : null,
    };
    const options = {
      organizationId,
      workspaceId,
      idempotencyKey: keyRef.current,
    };
    try {
      const result = selected.approvalRequired
        ? await createShareRequest(node.id, input, options)
        : await createGovernedGrant(node.id, input, options);
      setSuccess(isPending(result) ? "PENDING_APPROVAL" : "ACTIVE");
      keyRef.current = idempotencyKey();
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  function backOrClose() {
    if (step === 1) onClose();
    else setStep(1);
  }

  function closeUnlessSubmitting() {
    if (!submitting) onClose();
  }

  function renderFooter() {
    if (success) {
      return createElement(
        "button",
        { type: "button", className: "btn btn-primary", onClick: onClose },
        "完成",
      );
    }
    return createElement(
      "div",
      { className: "flex flex-wrap justify-end gap-2" },
      createElement(
        "button",
        {
          type: "button",
          className: "btn btn-secondary",
          disabled: submitting,
          onClick: backOrClose,
        },
        step === 1 ? "取消" : "上一步",
      ),
      createElement(
        "button",
        {
          type: "button",
          className: "btn btn-primary",
          disabled: step === 1
            ? !selected || Boolean(selected.disabledReason)
            : submitting || form.actions.length === 0,
          onClick: step === 1 ? () => setStep(2) : () => void submit(),
        },
        step === 1
          ? "下一步"
          : submitting
            ? "提交中…"
            : submitLabel(Boolean(selected?.approvalRequired)),
      ),
    );
  }

  function renderBody() {
    if (success) {
      return createElement(
        InlineAlert,
        {
          tone: "success",
          children: success === "ACTIVE" ? "共享已生效" : "等待审批",
        },
      );
    }
    if (step === 1) {
      return createElement(
        "fieldset",
        { className: "stack-sm" },
        createElement("legend", { className: "font-semibold" }, "受众"),
        ...audiences.map((option) => {
            const key = `${option.principalType}:${option.principalId}`;
            return createElement(
              "label",
              { key, className: "card stack-xs" },
              createElement(
                "span",
                { className: "flex items-center gap-2" },
                createElement("input", {
                  type: "radio",
                  name: "share-audience",
                  checked: selectedKey === key,
                  disabled: Boolean(option.disabledReason),
                  onChange: () => setSelectedKey(key),
                }),
                createElement("span", null, option.label),
              ),
              createElement(
                "span",
                { className: "text-sm text-muted" },
                option.disabledReason ?? impactSummary(option),
              ),
            );
          }),
      );
    }
    return createElement(
      "div",
      { className: "stack-lg" },
      createElement(
        "p",
        { className: "text-sm text-muted" },
        selected ? impactSummary(selected) : null,
      ),
      createElement(
        "fieldset",
        { className: "stack-sm" },
        createElement("legend", { className: "font-semibold" }, "权限"),
        createElement(
          "div",
          { className: "grid gap-2 sm:grid-cols-2" },
          ...actionOptions.map((action) =>
            createElement(
              "label",
              { key: action, className: "flex items-center gap-2" },
              createElement("input", {
                type: "checkbox",
                checked: form.actions.includes(action),
                onChange: () => toggleAction(action),
              }),
              createElement("span", null, action),
            ),
          ),
        ),
      ),
      node.type === "DIRECTORY"
        ? createElement(
            "label",
            { className: "flex items-center gap-2" },
            createElement("input", {
              type: "checkbox",
              checked: form.inheritToChildren,
              onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
                setForm((current) => ({
                  ...current,
                  inheritToChildren: event.currentTarget.checked,
                })),
            }),
            createElement("span", null, "应用到子项"),
          )
        : null,
      createElement(
        "label",
        { className: "stack-xs" },
        createElement("span", null, "到期时间（可选）"),
        createElement("input", {
          type: "datetime-local",
          className: "input",
          value: form.expiresAt,
          onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
            setForm((current) => ({ ...current, expiresAt: event.currentTarget.value })),
        }),
      ),
    );
  }

  return createElement(
    Modal,
    {
      open,
      onClose: closeUnlessSubmitting,
      title: `共享“${node.name}”`,
      description: step === 1 ? "选择可验证的受众" : "设置权限和到期时间",
      size: "lg",
      footer: renderFooter(),
      children: createElement(
        "div",
        { className: "stack-lg" },
        renderBody(),
        error ? createElement(InlineAlert, { tone: "error", children: error }) : null,
      ),
    },
  );
}
