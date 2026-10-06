"use client";

import { useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductVM, ReleasePlanTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { defaultReleaseWindow, fromDatetimeLocalValue, toDatetimeLocalValue } from "./release-plan-utils";

interface ReleasePlanFromTemplateModalProps {
  open: boolean;
  workspaceId: string;
  templates: ReleasePlanTemplateVM[];
  initialTemplateId?: string;
  onClose: () => void;
  onCreated: (productId: string, planId: string) => void;
}

export function ReleasePlanFromTemplateModal({
  open,
  workspaceId,
  templates,
  initialTemplateId,
  onClose,
  onCreated,
}: ReleasePlanFromTemplateModalProps) {
  const [products, setProducts] = useState<ProductVM[]>([]);
  const [templateId, setTemplateId] = useState("");
  const [productId, setProductId] = useState("");
  const [name, setName] = useState("");
  const [windowStart, setWindowStart] = useState("");
  const [windowEnd, setWindowEnd] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState(false);
  const [scheduleExecuteTime, setScheduleExecuteTime] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectedTemplate = templates.find((t) => t.id === templateId);

  useEffect(() => {
    if (!open) return;
    setError(null);
    const win = defaultReleaseWindow();
    const picked =
      templates.find((t) => t.id === initialTemplateId) ??
      templates.find((t) => t.id === templateId) ??
      templates[0];
    setTemplateId(picked?.id ?? "");
    setName(picked?.name ?? "");
    setWindowStart(toDatetimeLocalValue(win.start));
    setWindowEnd(toDatetimeLocalValue(win.end));
    setScheduleEnabled(false);
    setScheduleExecuteTime("");
    void product.listProducts(workspaceId).then((items) => {
      setProducts(items);
      setProductId((prev) => prev || items[0]?.id || "");
    }).catch(() => setProducts([]));
  }, [open, workspaceId, templates, initialTemplateId]);

  async function onSubmit() {
    if (!templateId || !productId) {
      setError("请选择模板与目标产品");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const created = await product.createReleasePlanFromTemplate(workspaceId, templateId, {
        product_id: productId,
        name: name.trim() || undefined,
        release_window_start: fromDatetimeLocalValue(windowStart),
        release_window_end: fromDatetimeLocalValue(windowEnd),
        schedule_execute_time: scheduleEnabled && scheduleExecuteTime ? fromDatetimeLocalValue(scheduleExecuteTime) : undefined,
      });
      onCreated(created.product_id, created.id);
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "从模板创建失败");
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
      title="从模板创建发布计划"
      description="模板已包含发布项结构，创建后可直接进入规划。"
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          onSubmit={() => void onSubmit()}
          submitLabel="创建计划"
          submittingLabel="创建中…"
          busy={busy}
          disabled={!templateId || !productId || templates.length === 0}
        />
      }
    >
      <div className="release-plan-form stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {templates.length === 0 ? (
          <p className="text-sm text-muted-foreground">暂无模板。可在产品发布计划详情中「存为模板」。</p>
        ) : (
          <>
            <section className="release-plan-form-section">
              <div className="release-plan-form-section-head">
                <h3 className="release-plan-form-section-title">模板与归属</h3>
              </div>
              <div className="release-plan-form-section-body stack-sm">
                <label className="form-field">
                  <span className="form-label">工作区模板 *</span>
                  <select
                    className="input"
                    value={templateId}
                    onChange={(e) => {
                      const id = e.target.value;
                      setTemplateId(id);
                      const tpl = templates.find((t) => t.id === id);
                      if (tpl) setName(tpl.name);
                    }}
                  >
                    {templates.map((tpl) => (
                      <option key={tpl.id} value={tpl.id}>
                        {tpl.name}（{tpl.job_count ?? 0} 项）
                      </option>
                    ))}
                  </select>
                  {selectedTemplate ? (
                    <p className="field-hint text-xs text-muted-foreground">
                      含 {selectedTemplate.job_count ?? 0} 个发布项
                      {selectedTemplate.description ? ` · ${selectedTemplate.description}` : ""}
                    </p>
                  ) : null}
                </label>
                <div className="grid gap-3 sm:grid-cols-2">
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
                    <span className="form-label">计划名称</span>
                    <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="默认使用模板名称" />
                  </label>
                </div>
              </div>
            </section>

            <section className="release-plan-form-section">
              <div className="release-plan-form-section-head">
                <h3 className="release-plan-form-section-title">发布窗口</h3>
              </div>
              <div className="release-plan-form-section-body stack-sm">
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="form-field">
                    <span className="form-label">开始时间</span>
                    <input className="input" type="datetime-local" value={windowStart} onChange={(e) => setWindowStart(e.target.value)} />
                  </label>
                  <label className="form-field">
                    <span className="form-label">结束时间</span>
                    <input className="input" type="datetime-local" value={windowEnd} onChange={(e) => setWindowEnd(e.target.value)} />
                  </label>
                </div>
                <label className="flex items-center gap-2 py-1">
                  <input
                    type="checkbox"
                    checked={scheduleEnabled}
                    onChange={(e) => {
                      setScheduleEnabled(e.target.checked);
                      if (!e.target.checked) setScheduleExecuteTime("");
                    }}
                  />
                  <span className="text-sm">启用定时自动执行</span>
                </label>
                {scheduleEnabled ? (
                  <label className="form-field">
                    <span className="form-label">定时执行时间</span>
                    <input className="input" type="datetime-local" value={scheduleExecuteTime} onChange={(e) => setScheduleExecuteTime(e.target.value)} />
                  </label>
                ) : null}
              </div>
            </section>
          </>
        )}
      </div>
    </Modal>
  );
}
