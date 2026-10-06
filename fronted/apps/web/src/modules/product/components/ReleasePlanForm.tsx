"use client";

import { useEffect, useState } from "react";
import type { ApprovalConfig, ReleasePlanVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanFormFields } from "./ReleasePlanFormFields";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
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

function emptyFormValues(): ReleasePlanFormValues {
  const d = defaultReleaseWindow();
  return {
    name: "",
    description: "",
    windowStart: toDatetimeLocalValue(d.start),
    windowEnd: toDatetimeLocalValue(d.end),
    managerId: "",
    approvalEnabled: true,
    approvalProvider: "internal",
    approvalWebhookUrl: "",
    scheduleExecuteTime: "",
  };
}

export function ReleasePlanForm({ open, title, workspaceId, busy, initial, onClose, onSubmit }: ReleasePlanFormProps) {
  const [form, setForm] = useState<ReleasePlanFormValues>(emptyFormValues);
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
        ...approval,
      });
      setManagerName("");
      return;
    }
    setForm(emptyFormValues());
    setManagerName("");
  }, [open, initial]);

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
        values={form}
        variant="sectioned"
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
  };
}
