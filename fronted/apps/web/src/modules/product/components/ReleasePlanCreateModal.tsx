"use client";

import { useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductVM, SprintSummaryVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { buildApprovalConfig, type ReleasePlanFormValues } from "./ReleasePlanForm";
import { ReleasePlanFormFields } from "./ReleasePlanFormFields";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { defaultReleaseWindow, fromDatetimeLocalValue, toDatetimeLocalValue } from "./release-plan-utils";

function emptyFormValues(): ReleasePlanFormValues {
  const win = defaultReleaseWindow();
  return {
    name: "",
    description: "",
    windowStart: toDatetimeLocalValue(win.start),
    windowEnd: toDatetimeLocalValue(win.end),
    managerId: "",
    approvalEnabled: false,
    approvalProvider: "internal",
    approvalWebhookUrl: "",
    scheduleExecuteTime: "",
  };
}

interface ReleasePlanCreateModalProps {
  open: boolean;
  workspaceId: string;
  defaultProductId?: string;
  onClose: () => void;
  onCreated: (productId: string, planId: string) => void;
}

export function ReleasePlanCreateModal({
  open,
  workspaceId,
  defaultProductId,
  onClose,
  onCreated,
}: ReleasePlanCreateModalProps) {
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [sprints, setSprints] = useState<SprintSummaryVM[]>([]);
  const [productId, setProductId] = useState("");
  const [sprintId, setSprintId] = useState("");
  const [form, setForm] = useState<ReleasePlanFormValues>(emptyFormValues);
  const [managerName, setManagerName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    setForm(emptyFormValues());
    setSprintId("");
    setManagerName("");
    void product.listProducts(workspaceId).then((items) => {
      setProducts(items);
      const picked = defaultProductId && items.some((p) => p.id === defaultProductId) ? defaultProductId : items[0]?.id ?? "";
      setProductId(picked);
    }).catch(() => setProducts([]));
  }, [open, workspaceId, defaultProductId]);

  useEffect(() => {
    if (!open || !productId) {
      setSprints([]);
      setSprintId("");
      return;
    }
    void product.listSprints(workspaceId, productId).then(setSprints).catch(() => setSprints([]));
  }, [open, workspaceId, productId]);

  async function onSubmit() {
    if (!productId) {
      setError("请选择目标产品");
      return;
    }
    if (!form.name.trim()) {
      setError("请填写计划名称");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const created = await product.createReleasePlan(workspaceId, productId, {
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        release_window_start: fromDatetimeLocalValue(form.windowStart),
        release_window_end: fromDatetimeLocalValue(form.windowEnd),
        approval_enabled: form.approvalEnabled,
        approval_config: buildApprovalConfig(form),
        schedule_execute_time: form.scheduleExecuteTime ? fromDatetimeLocalValue(form.scheduleExecuteTime) : undefined,
        manager_id: form.managerId || undefined,
        sprint_id: sprintId || undefined,
      });
      onCreated(productId, created.id);
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="lg"
      panelClassName="release-plan-form-modal"
      title="新建发布计划"
      description="先创建计划骨架，进入详情后再添加发布项。"
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          onSubmit={() => void onSubmit()}
          submitLabel="创建计划"
          submittingLabel="创建中…"
          busy={busy}
          disabled={!productId || !form.name.trim()}
        />
      }
    >
      <div className="release-plan-form stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        <section className="release-plan-form-section">
          <div className="release-plan-form-section-head">
            <h3 className="release-plan-form-section-title">归属</h3>
            <p className="release-plan-form-section-hint">选择计划所属产品与迭代</p>
          </div>
          <div className="release-plan-form-section-body grid gap-3 sm:grid-cols-2">
            <label className="form-field">
              <span className="form-label">目标产品 *</span>
              <select className="input" value={productId} onChange={(e) => setProductId(e.target.value)}>
                {products.length === 0 ? <option value="">暂无产品</option> : null}
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.display_name || p.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span className="form-label">关联迭代</span>
              <select className="input" value={sprintId} onChange={(e) => setSprintId(e.target.value)}>
                <option value="">不关联</option>
                {sprints.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
        </section>

        <ReleasePlanFormFields
          workspaceId={workspaceId}
          values={form}
          variant="sectioned"
          onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
          managerName={managerName}
          onManagerNameChange={setManagerName}
        />
      </div>
    </Modal>
  );
}
