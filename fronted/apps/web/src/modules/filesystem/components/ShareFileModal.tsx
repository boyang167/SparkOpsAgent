"use client";

import { createElement, useEffect, useMemo, useRef, useState } from "react";
import type { ReactElement } from "react";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import type {
  CreateGovernedGrantCommand,
  FileNodeVM,
  GovernedPermission,
  GovernedShareSubmissionVM,
  SharePreviewVM,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  createGovernedGrant,
  createShareRequest,
  previewGovernedShare,
} from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  allowedActions,
  canonicalSharePayload,
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
  const [previewing, setPreviewing] = useState(false);
  const [preview, setPreview] = useState<SharePreviewVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<SuccessState | null>(null);
  const submitKeyRef = useRef<{ fingerprint: string; key: string } | null>(null);
  const previewKeyRef = useRef<{ fingerprint: string; key: string } | null>(null);
  const submitInFlightRef = useRef(false);
  const classification = node.classification ?? "RESTRICTED";

  const audiences = useMemo<ShareAudienceOption[]>(() => [
    {
      principalType: "ORGANIZATION",
      principalId: organizationId,
      label: organizationLabel,
      disabledReason: classification === "RESTRICTED"
        ? "受限内容不能共享给整个组织"
        : undefined,
    },
    {
      principalType: "WORKSPACE",
      principalId: workspaceId,
      label: workspaceLabel,
      memberCount: members.length,
      disabledReason: memberDirectoryAvailable
        ? undefined
        : "工作区成员目录不可用，无法提供准确影响人数",
    },
    ...members.map((member) => ({
      principalType: "USER" as const,
      principalId: member.member_id,
      label: memberLabel(member),
      memberCount: 1,
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
  const input = useMemo<CreateGovernedGrantCommand | null>(() => selected
    ? {
        principal_type: selected.principalType,
        principal_ids: [selected.principalId],
        actions: form.actions,
        effect: form.effect,
        inherit_to_children: node.type === "DIRECTORY" ? form.inheritToChildren : false,
        expires_at: form.expiresAt ? new Date(form.expiresAt).toISOString() : null,
      }
    : null, [form, node.type, selected]);
  const fingerprint = input ? canonicalSharePayload(input) : "";

  useEffect(() => {
    if (!open) return;
    setStep(1);
    setSelectedKey("");
    setForm(createInitialShareForm(node.type));
    setSubmitting(false);
    setPreviewing(false);
    setPreview(null);
    setError(null);
    setSuccess(null);
    submitKeyRef.current = null;
    previewKeyRef.current = null;
    submitInFlightRef.current = false;
  }, [node.id, node.type, open]);

  useEffect(() => {
    if (!open || step !== 2 || !input || input.actions.length === 0) {
      setPreview(null);
      setPreviewing(false);
      return;
    }
    if (!effectiveActions.includes("SHARE")) {
      setPreview(null);
      setPreviewing(false);
      setError("无权共享此内容。");
      return;
    }
    const keyRecord = previewKeyRef.current?.fingerprint === fingerprint
      ? previewKeyRef.current
      : { fingerprint, key: idempotencyKey() };
    previewKeyRef.current = keyRecord;
    let stale = false;
    setPreview(null);
    setPreviewing(true);
    setError(null);
    void previewGovernedShare(node.id, input, {
      organizationId,
      workspaceId,
      idempotencyKey: keyRecord.key,
    }).then((result) => {
      if (!stale) setPreview(result);
    }).catch((previewError) => {
      if (!stale) setError(errorMessage(previewError));
    }).finally(() => {
      if (!stale) setPreviewing(false);
    });
    return () => {
      stale = true;
    };
  }, [
    effectiveActions,
    fingerprint,
    input,
    node.id,
    open,
    organizationId,
    step,
    workspaceId,
  ]);

  function toggleAction(action: GovernedPermission) {
    setForm((current) => ({
      ...current,
      actions: current.actions.includes(action)
        ? current.actions.filter((item) => item !== action)
        : [...current.actions, action],
    }));
  }

  async function submit() {
    if (submitInFlightRef.current) return;
    if (!selected || selected.disabledReason || !input || form.actions.length === 0 || !preview) return;
    if (!effectiveActions.includes("SHARE")) {
      setError("无权共享此内容。");
      return;
    }
    submitInFlightRef.current = true;
    setSubmitting(true);
    setError(null);
    if (!submitKeyRef.current || submitKeyRef.current.fingerprint !== fingerprint) {
      submitKeyRef.current = { fingerprint, key: idempotencyKey() };
    }
    const options = {
      organizationId,
      workspaceId,
      idempotencyKey: submitKeyRef.current.key,
    };
    try {
      const result = preview.outcome === "PENDING_APPROVAL"
        ? await createShareRequest(node.id, input, options)
        : await createGovernedGrant(node.id, input, options);
      setSuccess(isPending(result) ? "PENDING_APPROVAL" : "ACTIVE");
      submitKeyRef.current = null;
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      submitInFlightRef.current = false;
      setSubmitting(false);
    }
  }

  function resetAndClose() {
    submitKeyRef.current = null;
    previewKeyRef.current = null;
    submitInFlightRef.current = false;
    onClose();
  }

  function backOrClose() {
    if (step === 1) resetAndClose();
    else setStep(1);
  }

  function closeUnlessSubmitting() {
    if (!submitting) resetAndClose();
  }

  function renderFooter() {
    if (success) {
      return createElement(
        "button",
        { type: "button", className: "btn btn-primary", onClick: resetAndClose },
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
            : submitting || previewing || !preview ||
              !effectiveActions.includes("SHARE") || form.actions.length === 0,
          onClick: step === 1 ? () => setStep(2) : () => void submit(),
        },
        step === 1
          ? "下一步"
          : submitting
            ? "提交中…"
            : previewing
              ? "检查策略…"
              : submitLabel(preview?.outcome === "PENDING_APPROVAL"),
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
                  onChange: () => {
                    setSelectedKey(key);
                    const allowed = allowedActions(option.principalType);
                    setForm((current) => ({
                      ...current,
                      actions: current.actions.filter((action) => allowed.includes(action)),
                    }));
                  },
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
