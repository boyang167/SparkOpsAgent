"use client";

import { useEffect, useState } from "react";
import type { ApprovalConfig, ReleasePlanVM, SprintExternalRef } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { useWorkspaceMembers } from "../hooks/useWorkspaceMembers";
import { ReleasePlanFormFields } from "./ReleasePlanFormFields";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import {
  buildHookSettingsPayload,
  emptyHookSettingsFormValues,
  readHookSettingsFormValues,
  type HookSettingsFormValues,
} from "./ReleasePlanHookSettingsField";
import { defaultReleaseWindow, fromDatetimeLocalValue, toDatetimeLocalValue } from "./release-plan-utils";

export interface ReleasePlanSprintOption {
  id: string;
  name: string;
  external?: SprintExternalRef;
}

export interface ReleasePlanFormValues {
  name: string;
  description: string;
  windowStart: string;
  windowEnd: string;
  managerId: string;
  sprintId: string;
  approverIds: string[];
  approvalEnabled: boolean;
  approvalProvider: "internal" | "feishu" | "dingtalk";
  approvalWebhookUrl: string;
  scheduleExecuteTime: string;
  hookSettings: HookSettingsFormValues;
}

interface ReleasePlanFormProps {
  open: boolean;
  title: string;
  workspaceId: string;
  productId?: string;
  busy?: boolean;
  initial?: ReleasePlanVM | null;
  sprintOptions?: ReleasePlanSprintOption[];
  sprintLocked?: boolean;
  defaultSprintId?: string;
  onSprintOptionsChange?: (options: ReleasePlanSprintOption[]) => void;
  onClose: () => void;
  onSubmit: (values: ReleasePlanFormValues) => void | Promise<void>;
}

function readApprovalConfig(
  plan?: ReleasePlanVM | null,
): Pick<ReleasePlanFormValues, "approvalEnabled" | "approvalProvider" | "approvalWebhookUrl" | "approverIds"> {
  const cfg = plan?.approval_config;
  return {
    approvalEnabled: cfg?.enabled ?? plan?.approval_enabled ?? true,
    approvalProvider: (cfg?.provider as ReleasePlanFormValues["approvalProvider"]) ?? "internal",
    approvalWebhookUrl: cfg?.webhook_url ?? "",
    approverIds: cfg?.approver_ids ?? [],
  };
}

export function emptyReleasePlanFormValues(): ReleasePlanFormValues {
  const d = defaultReleaseWindow();
  return {
    name: "",
    description: "",
    windowStart: toDatetimeLocalValue(d.start),
    windowEnd: toDatetimeLocalValue(d.end),
    managerId: "",
    sprintId: "",
    approverIds: [],
    approvalEnabled: true,
    approvalProvider: "internal",
    approvalWebhookUrl: "",
    scheduleExecuteTime: "",
    hookSettings: emptyHookSettingsFormValues(),
  };
}

export function ReleasePlanForm({
  open,
  title,
  workspaceId,
  productId,
  busy,
  initial,
  sprintOptions,
  sprintLocked,
  defaultSprintId,
  onSprintOptionsChange,
  onClose,
  onSubmit,
}: ReleasePlanFormProps) {
  const { memberName } = useWorkspaceMembers(workspaceId);
  const [form, setForm] = useState<ReleasePlanFormValues>(emptyReleasePlanFormValues);
  const [managerName, setManagerName] = useState("");

  useEffect(() => {
    if (!open) return;
    if (initial) {
      const approval = readApprovalConfig(initial);
      setForm({
        name: initial.name,
        description: initial.description ?? "",
        windowStart: toDatetimeLocalValue(initial.release_window_start),
        windowEnd: toDatetimeLocalValue(initial.release_window_end),
        scheduleExecuteTime: initial.schedule_execute_time ? toDatetimeLocalValue(initial.schedule_execute_time) : "",
        managerId: initial.manager_id ?? "",
        sprintId: initial.sprint_id ?? "",
        hookSettings: readHookSettingsFormValues(initial.hook_settings),
        ...approval,
      });
      setManagerName(initial.manager_id ? memberName(initial.manager_id) : "");
      return;
    }
    const empty = emptyReleasePlanFormValues();
    setForm({
      ...empty,
      sprintId: defaultSprintId ?? "",
    });
    setManagerName("");
  }, [open, initial, memberName, defaultSprintId]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="lg"
      panelClassName="release-plan-form-modal"
      title={title}
      description="配置发布窗口、负责人、审批与定时执行。"
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          busy={busy}
          disabled={!form.name.trim()}
          onSubmit={() =>
            void onSubmit({
              ...form,
              name: form.name.trim(),
              description: form.description.trim(),
              windowStart: fromDatetimeLocalValue(form.windowStart),
              windowEnd: fromDatetimeLocalValue(form.windowEnd),
              approvalWebhookUrl: form.approvalWebhookUrl.trim(),
              scheduleExecuteTime: form.scheduleExecuteTime ? fromDatetimeLocalValue(form.scheduleExecuteTime) : "",
            })
          }
        />
      }
    >
      <ReleasePlanFormFields
        workspaceId={workspaceId}
        productId={productId}
        values={form}
        variant="sectioned"
        sprintOptions={sprintOptions}
        sprintLocked={sprintLocked}
        onSprintOptionsChange={onSprintOptionsChange}
        onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
        managerName={managerName}
        onManagerNameChange={setManagerName}
      />
    </Modal>
  );
}

export function buildApprovalConfig(values: ReleasePlanFormValues): ApprovalConfig {
  return {
    enabled: values.approvalEnabled,
    provider: values.approvalProvider,
    webhook_url: values.approvalWebhookUrl || undefined,
    approver_ids: values.approverIds.length > 0 ? values.approverIds : undefined,
  };
}

export function buildHookSettingsConfig(values: ReleasePlanFormValues) {
  return buildHookSettingsPayload(values.hookSettings);
}
