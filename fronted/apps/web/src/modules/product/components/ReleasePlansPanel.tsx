"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";

interface ReleasePlansPanelProps {
  workspaceId: string;
  productId: string;
}

function defaultWindow() {
  const start = new Date();
  const end = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000);
  return { start: start.toISOString(), end: end.toISOString() };
}

function statusTone(status: string): string {
  if (status === "success" || status === "done") return "text-green-600";
  if (status === "failed" || status === "denied") return "text-red-600";
  if (status === "running" || status === "executing") return "text-blue-600";
  return "text-muted-foreground";
}

export function ReleasePlansPanel({ workspaceId, productId }: ReleasePlansPanelProps) {
  const [items, setItems] = useState<ReleasePlanVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selected, setSelected] = useState<ReleasePlanVM | null>(null);
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState("");
  const [detailTab, setDetailTab] = useState<"jobs" | "audit">("jobs");
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await product.listReleasePlans(workspaceId, productId);
      setItems(list);
      setSelectedId((prev) => {
        if (prev && list.some((item) => item.id === prev)) return prev;
        return list[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载发布计划失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadDetail = useCallback(
    async (planId: string) => {
      try {
        setSelected(await product.getReleasePlan(workspaceId, productId, planId));
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载详情失败");
        setSelected(null);
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    if (!selectedId) {
      setSelected(null);
      return;
    }
    void loadDetail(selectedId);
  }, [selectedId, loadDetail]);

  const loadWorkflows = useCallback(async () => {
    try {
      const wfItems = await product.listProductWorkflows(workspaceId, productId);
      setWorkflows(wfItems);
      if (wfItems.length > 0) {
        setSelectedWorkflowId((prev) => prev || wfItems[0].id);
      }
    } catch {
      setWorkflows([]);
    }
  }, [workspaceId, productId]);

  const needsPolling =
    selected?.status === "executing" &&
    selected.jobs?.some((job) => job.status === "running" || job.type === "workflow");

  useEffect(() => {
    if (!selected || !needsPolling) return;
    const timer = window.setInterval(() => {
      void loadDetail(selected.id);
      void load();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [selected, needsPolling, load, loadDetail]);

  useEffect(() => {
    if (selected?.status === "planning") {
      void loadWorkflows();
    }
  }, [selected?.status, loadWorkflows]);

  async function refreshDetail() {
    if (!selectedId) return;
    await loadDetail(selectedId);
    await load();
  }

  async function onCreate() {
    if (!name.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const win = defaultWindow();
      const created = await product.createReleasePlan(workspaceId, productId, {
        name: name.trim(),
        release_window_start: win.start,
        release_window_end: win.end,
        approval_enabled: true,
      });
      setName("");
      setCreateOpen(false);
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  async function onAddTextJob() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.addReleasePlanJob(workspaceId, productId, selected.id, {
        name: `检查项 ${(selected.jobs?.length ?? 0) + 1}`,
        type: "text",
        text_content: "发布前检查清单",
        sort_order: selected.jobs?.length ?? 0,
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onAddWorkflowJob() {
    if (!selected || !selectedWorkflowId) return;
    setBusy(true);
    try {
      const wf = workflows.find((w) => w.id === selectedWorkflowId);
      await product.addReleasePlanJob(workspaceId, productId, selected.id, {
        name: wf?.name ?? "工作流发布",
        type: "workflow",
        sort_order: selected.jobs?.length ?? 0,
        workflow_ref: { mode: "reference", workflow_id: selectedWorkflowId },
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加工作流发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onFinishPlanning() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.finishReleasePlanPlanning(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "完成规划失败");
    } finally {
      setBusy(false);
    }
  }

  async function onApprove() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.approveReleasePlan(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "审批失败");
    } finally {
      setBusy(false);
    }
  }

  async function onExecute() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.executeReleasePlan(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "执行失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobDone(jobId: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.markReleasePlanJobDone(workspaceId, productId, selected.id, jobId);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobSkip(jobId: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.skipReleasePlanJob(workspaceId, productId, selected.id, jobId);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "跳过失败");
    } finally {
      setBusy(false);
    }
  }

  async function onJobRetry(jobId: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.retryReleasePlanJob(workspaceId, productId, selected.id, jobId);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重试失败");
    } finally {
      setBusy(false);
    }
  }

  function renderJobActions(job: ReleasePlanJobVM) {
    if (selected?.status !== "executing") return null;
    if (job.type === "text" && job.status !== "done" && job.status !== "skipped") {
      return (
        <div className="flex gap-1">
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobDone(job.id)}>
            完成
          </button>
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobSkip(job.id)}>
            跳过
          </button>
        </div>
      );
    }
    if (job.type === "workflow" && job.status === "failed") {
      return (
        <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobRetry(job.id)}>
          重试
        </button>
      );
    }
    if (job.type === "workflow" && job.status !== "done" && job.status !== "skipped") {
      return (
        <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onJobSkip(job.id)}>
          跳过
        </button>
      );
    }
    return null;
  }

  const listItems = useMemo(
    () =>
      items.map((row) => ({
        id: row.id,
        title: row.name,
        meta: row.status,
      })),
    [items],
  );

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="迭代计划"
        searchPlaceholder="搜索计划"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={() => setCreateOpen(true)}
        addAriaLabel="新建发布计划"
        loading={loading}
        showMain={Boolean(selected)}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无发布计划，请点击 + 新建"
            description="创建迭代计划后，可规划发布项、提交审批并执行发布。"
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateOpen(true)}>
                新建发布计划
              </button>
            }
          />
        }
      >
        {selected ? (
          <div className="product-master-main-inner stack-md">
            <div className="flex justify-between items-start gap-3">
              <div className="stack-xs">
                <h2 className="section-title">{selected.name}</h2>
                {selected.description ? <p className="text-sm text-muted-foreground">{selected.description}</p> : null}
                <p className="text-xs text-muted-foreground">
                  发布窗口：{new Date(selected.release_window_start).toLocaleString()} —{" "}
                  {new Date(selected.release_window_end).toLocaleString()}
                </p>
              </div>
              <span className={`text-sm font-medium ${statusTone(selected.status)}`}>{selected.status}</span>
            </div>

            <ol className="flex flex-wrap gap-2 text-xs">
              {(["planning", "wait_for_approval", "executing", "success"] as const).map((step) => {
                const active = selected.status === step;
                const done =
                  (step === "planning" && selected.status !== "planning") ||
                  (step === "wait_for_approval" &&
                    !["planning", "wait_for_approval"].includes(selected.status) &&
                    selected.approval_enabled) ||
                  (step === "executing" && ["executing", "success"].includes(selected.status)) ||
                  (step === "success" && selected.status === "success");
                return (
                  <li
                    key={step}
                    className={`rounded px-2 py-1 border ${
                      active ? "border-primary bg-primary/5 text-primary" : done ? "border-green-600/30 text-green-700" : "text-muted-foreground"
                    }`}
                  >
                    {step}
                  </li>
                );
              })}
            </ol>

            <div className="flex flex-wrap gap-2">
              {selected.status === "planning" ? (
                <>
                  <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onAddTextJob()}>
                    添加 text 项
                  </button>
                  {workflows.length > 0 ? (
                    <>
                      <select
                        className="input text-xs"
                        value={selectedWorkflowId}
                        onChange={(e) => setSelectedWorkflowId(e.target.value)}
                      >
                        {workflows.map((wf) => (
                          <option key={wf.id} value={wf.id}>
                            {wf.name}
                          </option>
                        ))}
                      </select>
                      <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => void onAddWorkflowJob()}>
                        添加 workflow 项
                      </button>
                    </>
                  ) : (
                    <span className="text-xs text-muted-foreground">请先在「工作流」Tab 创建工作流</span>
                  )}
                  <button type="button" className="btn text-xs" disabled={busy} onClick={() => void onFinishPlanning()}>
                    完成规划
                  </button>
                </>
              ) : null}
              {selected.status === "wait_for_approval" ? (
                <button type="button" className="btn text-xs" disabled={busy} onClick={() => void onApprove()}>
                  审批通过
                </button>
              ) : null}
              {selected.status === "executing" ? (
                <button type="button" className="btn text-xs" disabled={busy} onClick={() => void onExecute()}>
                  继续执行
                </button>
              ) : null}
            </div>

            <div className="flex gap-2 text-xs border-b pb-2">
              <button
                type="button"
                className={detailTab === "jobs" ? "font-medium text-primary" : "text-muted-foreground"}
                onClick={() => setDetailTab("jobs")}
              >
                发布项
              </button>
              <button
                type="button"
                className={detailTab === "audit" ? "font-medium text-primary" : "text-muted-foreground"}
                onClick={() => setDetailTab("audit")}
              >
                操作记录
              </button>
            </div>

            {detailTab === "jobs" ? (
              <ul className="stack-xs text-sm">
                {(selected.jobs ?? []).map((job) => (
                  <li key={job.id} className="flex justify-between gap-2 border-b py-2">
                    <div className="stack-xs">
                      <span>
                        {job.name}{" "}
                        <span className="text-muted-foreground">({job.type})</span>{" "}
                        <span className={statusTone(job.status)}>{job.status}</span>
                      </span>
                      {job.type === "workflow" && job.workflow_ref?.workflow_id ? (
                        <span className="text-xs text-muted-foreground">
                          工作流 {job.workflow_ref.workflow_id.slice(0, 8)}…
                        </span>
                      ) : null}
                      {job.execution_ref?.workflow_run_id ? (
                        <span className="text-xs text-muted-foreground font-mono">
                          Run {job.execution_ref.workflow_run_id.slice(0, 8)}…
                        </span>
                      ) : null}
                      {job.execution_ref?.error_message ? (
                        <span className="text-xs text-red-600">{job.execution_ref.error_message}</span>
                      ) : null}
                    </div>
                    {renderJobActions(job)}
                  </li>
                ))}
              </ul>
            ) : (
              <ul className="stack-xs text-xs max-h-64 overflow-auto">
                {(selected.audit_logs ?? []).length === 0 ? (
                  <li className="text-muted-foreground">暂无操作记录</li>
                ) : (
                  (selected.audit_logs ?? []).map((log, i) => (
                    <li key={String(log.id ?? i)} className="border-b py-1">
                      <span className="font-medium">{String(log.action ?? "-")}</span>
                      {log.created_at ? (
                        <span className="text-muted-foreground ml-2">{new Date(String(log.created_at)).toLocaleString()}</span>
                      ) : null}
                    </li>
                  ))
                )}
              </ul>
            )}
          </div>
        ) : null}
      </ProductMasterDetail>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建发布计划"
        description="创建迭代发布计划，规划发布窗口与发布项。"
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy || !name.trim()} onClick={() => void onCreate()}>
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <label className="stack-xs text-sm">
          <span className="text-muted-foreground">计划名称</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="例如 v2.4 迭代发布" />
        </label>
      </Modal>
    </>
  );
}
