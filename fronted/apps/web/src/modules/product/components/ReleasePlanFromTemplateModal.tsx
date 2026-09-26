"use client";

import { useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductVM, ReleasePlanTemplateVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
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
  const [scheduleExecuteTime, setScheduleExecuteTime] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
        schedule_execute_time: scheduleExecuteTime ? fromDatetimeLocalValue(scheduleExecuteTime) : undefined,
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
      title="从模板创建发布计划"
      description="选择工作区模板与目标产品，快速生成可编辑的发布计划。"
      footer={
        <>
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={busy || !templateId || !productId}
            onClick={() => void onSubmit()}
          >
            {busy ? "创建中…" : "创建计划"}
          </button>
        </>
      }
    >
      <div className="stack-md text-sm">
        {error ? <p className="text-red-600 text-xs">{error}</p> : null}
        {templates.length === 0 ? (
          <p className="text-muted-foreground">暂无模板。可在产品发布计划详情中「存为模板」。</p>
        ) : (
          <>
            <label className="stack-xs">
              <span className="text-muted-foreground">模板</span>
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
            </label>
            <label className="stack-xs">
              <span className="text-muted-foreground">目标产品</span>
              <select className="input" value={productId} onChange={(e) => setProductId(e.target.value)}>
                {products.length === 0 ? <option value="">暂无产品</option> : null}
                {products.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.display_name || p.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="stack-xs">
              <span className="text-muted-foreground">计划名称</span>
              <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="默认使用模板名称" />
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
            </label>
          </>
        )}
      </div>
    </Modal>
  );
}
