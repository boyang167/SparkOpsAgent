"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProductWorkflowVM, ReleasePlanJobVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";
import { ReleasePlanForm, buildApprovalConfig, type ReleasePlanFormValues } from "./ReleasePlanForm";
import { ReleasePlanJobEditor } from "./ReleasePlanJobEditor";
import { ReleasePlanStatusSteps } from "./ReleasePlanStatusSteps";
import { SprintWorkItemsSummary } from "./SprintWorkItemsSummary";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { formatReleaseWindow, releasePlanStatusBadgeClass, releasePlanStatusLabel } from "./release-plan-utils";

interface ReleasePlansPanelProps {
  workspaceId: string;
  productId: string;
  sprintId?: string;
  embedded?: boolean;
  initialPlanId?: string | null;
}

export function ReleasePlansPanel({ workspaceId, productId, sprintId, embedded, initialPlanId }: ReleasePlansPanelProps) {
  const [items, setItems] = useState<ReleasePlanVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(initialPlanId ?? null);
  const [selected, setSelected] = useState<ReleasePlanVM | null>(null);
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [detailTab, setDetailTab] = useState<"jobs" | "audit">("jobs");
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [cancelOpen, setCancelOpen] = useState(false);
  const [jobDoneTarget, setJobDoneTarget] = useState<string | null>(null);
  const [jobDoneRemark, setJobDoneRemark] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (options?: { silent?: boolean }) => {
    if (!options?.silent) {
      setLoading(true);
      setError(null);
    }
    try {
      const list = await product.listReleasePlans(workspaceId, productId, sprintId);
      setItems(list);
      setSelectedId((prev) => {
        if (initialPlanId && list.some((item) => item.id === initialPlanId)) return initialPlanId;
        if (prev && list.some((item) => item.id === prev)) return prev;
        return list[0]?.id ?? null;
      });
    } catch (err) {
      if (!options?.silent) {
        setError(err instanceof BffError ? err.message : "加载发布计划失败");
      }
    } finally {
      if (!options?.silent) setLoading(false);
    }
  }, [workspaceId, productId, sprintId, initialPlanId]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadDetail = useCallback(
    async (planId: string) => {
      try {
        const detail = await product.getReleasePlan(workspaceId, productId, planId);
        setSelected(detail);
        setItems((prev) =>
          prev.map((item) =>
            item.id === planId ? { ...item, name: detail.name, status: detail.status } : item,
          ),
        );
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
    } catch {
      setWorkflows([]);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void loadWorkflows();
  }, [loadWorkflows]);

  const needsPolling =
    selected?.status === "executing" &&
    selected.jobs?.some((job) => job.status === "running" || job.type === "workflow");

  useEffect(() => {
    if (!selected || !needsPolling) return;
    const timer = window.setInterval(() => {
      void loadDetail(selected.id);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [selected, needsPolling, loadDetail]);

  async function refreshDetail(options?: { reloadList?: boolean }) {
    if (!selectedId) return;
    await loadDetail(selectedId);
    if (options?.reloadList) await load({ silent: true });
  }

  async function onCreate(values: ReleasePlanFormValues) {
    setBusy(true);
    setError(null);
    try {
      const created = await product.createReleasePlan(workspaceId, productId, {
        name: values.name,
        description: values.description || undefined,
        release_window_start: values.windowStart,
        release_window_end: values.windowEnd,
        approval_enabled: values.approvalEnabled,
        approval_config: buildApprovalConfig(values),
        schedule_execute_time: values.scheduleExecuteTime || undefined,
        manager_id: values.managerId || undefined,
        ...(sprintId ? { sprint_id: sprintId } : {}),
      });
      setCreateOpen(false);
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  async function onUpdate(values: ReleasePlanFormValues) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.updateReleasePlan(workspaceId, productId, selected.id, {
        name: values.name,
        description: values.description || undefined,
        release_window_start: values.windowStart,
        release_window_end: values.windowEnd,
        approval_enabled: values.approvalEnabled,
        approval_config: buildApprovalConfig(values),
        schedule_execute_time: values.scheduleExecuteTime || undefined,
        clear_schedule_time: !values.scheduleExecuteTime,
        manager_id: values.managerId || undefined,
      });
      setEditOpen(false);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新失败");
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
        text_content: "<p>发布前检查清单</p>",
        text_format: "html",
        sort_order: selected.jobs?.length ?? 0,
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onAddWorkflowJob(workflowId: string) {
    if (!selected || !workflowId) return;
    setBusy(true);
    try {
      const wf = workflows.find((w) => w.id === workflowId);
      await product.addReleasePlanJob(workspaceId, productId, selected.id, {
        name: wf?.name ?? "工作流发布",
        type: "workflow",
        sort_order: selected.jobs?.length ?? 0,
        workflow_ref: { mode: "reference", workflow_id: workflowId },
      });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加工作流发布项失败");
    } finally {
      setBusy(false);
    }
  }

  async function onMoveJob(jobId: string, direction: "up" | "down") {
    if (!selected?.jobs) return;
    const sorted = [...selected.jobs].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
    const idx = sorted.findIndex((j) => j.id === jobId);
    if (idx < 0) return;
    const swap = direction === "up" ? idx - 1 : idx + 1;
    if (swap < 0 || swap >= sorted.length) return;
    const ids = sorted.map((j) => j.id);
    [ids[idx], ids[swap]] = [ids[swap], ids[idx]];
    setBusy(true);
    try {
      await product.reorderReleasePlanJobs(workspaceId, productId, selected.id, ids);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "排序失败");
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

  async function onReject() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.rejectReleasePlan(workspaceId, productId, selected.id, { reason: rejectReason.trim() || undefined });
      setRejectOpen(false);
      setRejectReason("");
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "驳回失败");
    } finally {
      setBusy(false);
    }
  }

  async function onReopen() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.reopenReleasePlanPlanning(workspaceId, productId, selected.id);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重新编辑失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCancel() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.cancelReleasePlan(workspaceId, productId, selected.id);
      setCancelOpen(false);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "取消失败");
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

  async function onJobDone(jobId: string, remark?: string) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.markReleasePlanJobDone(workspaceId, productId, selected.id, jobId, { remark: remark?.trim() || undefined });
      setJobDoneTarget(null);
      setJobDoneRemark("");
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

  async function onGenerateFromWorkItems(workItemIds: string[]) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.addReleasePlanJobsFromWorkItems(workspaceId, productId, selected.id, workItemIds);
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "生成检查项失败");
    } finally {
      setBusy(false);
    }
  }

  function renderJobActions(job: ReleasePlanJobVM) {
    if (selected?.status !== "executing") return null;
    if (job.type === "text" && job.status !== "done" && job.status !== "skipped") {
      return (
        <div className="flex gap-1">
          <button type="button" className="btn-secondary text-xs" disabled={busy} onClick={() => setJobDoneTarget(job.id)}>
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
        meta: releasePlanStatusLabel(row.status),
        metaClassName: `badge ${releasePlanStatusBadgeClass(row.status)} product-master-item-badge`,
      })),
    [items],
  );

  const canEditJobs = selected?.status === "planning";
  const usesExternalApproval =
    selected?.approval_config?.provider === "feishu" || selected?.approval_config?.provider === "dingtalk";

  async function onCopyPlan() {
    if (!selected) return;
    setBusy(true);
    try {
      const copied = await product.copyReleasePlan(workspaceId, productId, selected.id, {});
      await load();
      setSelectedId(copied.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "复制失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveTemplate() {
    if (!selected) return;
    setBusy(true);
    try {
      await product.saveReleasePlanTemplate(workspaceId, productId, selected.id, { name: `${selected.name} 模板` });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存模板失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle={embedded ? "本迭代发布" : "发布计划"}
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
            title={embedded ? "本迭代暂无发布计划" : "暂无发布计划，请点击 + 新建"}
            description={
              embedded
                ? "为本迭代创建发布计划，规划发布项并执行上线。"
                : "创建迭代计划后，可规划发布项、提交审批并执行发布。"
            }
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateOpen(true)}>
                新建发布计划
              </button>
            }
          />
        }
      >
        {selected ? (
          <div className="product-master-main-inner release-plan-detail stack-md">
            <header className="release-plan-detail-header">
              <div className="release-plan-detail-head">
                <div className="stack-xs min-w-0">
                  <h2 className="section-title">{selected.name}</h2>
                  {selected.description ? <p className="text-sm text-muted-foreground">{selected.description}</p> : null}
                </div>
                <span className={`badge shrink-0 ${releasePlanStatusBadgeClass(selected.status)}`}>
                  {releasePlanStatusLabel(selected.status)}
                </span>
              </div>
              <dl className="release-plan-detail-meta">
                <div className="release-plan-detail-meta-item">
                  <dt>发布窗口</dt>
                  <dd>{formatReleaseWindow(selected.release_window_start, selected.release_window_end)}</dd>
                </div>
                {selected.schedule_execute_time ? (
                  <div className="release-plan-detail-meta-item">
                    <dt>定时执行</dt>
                    <dd>{new Date(selected.schedule_execute_time).toLocaleString("zh-CN")}</dd>
                  </div>
                ) : null}
                {usesExternalApproval ? (
                  <div className="release-plan-detail-meta-item">
                    <dt>外部审批</dt>
                    <dd>
                      {selected.approval_config?.provider === "feishu" ? "飞书" : "钉钉"}
                      {selected.approval_config?.instance?.status ? ` · ${selected.approval_config.instance.status}` : ""}
                    </dd>
                  </div>
                ) : null}
              </dl>
            </header>

            <ReleasePlanStatusSteps status={selected.status} approvalEnabled={selected.approval_enabled} />

            <div className="release-plan-detail-actions">
              <div className="release-plan-detail-actions-primary">
                {selected.status === "planning" ? (
                  <>
                    <button type="button" className="btn btn-primary text-xs" disabled={busy} onClick={() => void onFinishPlanning()}>
                      完成规划
                    </button>
                    <button type="button" className="btn btn-secondary text-xs" disabled={busy} onClick={() => setEditOpen(true)}>
                      编辑
                    </button>
                  </>
                ) : null}
                {selected.status === "wait_for_approval" && !usesExternalApproval ? (
                  <>
                    <button type="button" className="btn btn-primary text-xs" disabled={busy} onClick={() => void onApprove()}>
                      审批通过
                    </button>
                    <button type="button" className="btn btn-secondary text-xs" disabled={busy} onClick={() => setRejectOpen(true)}>
                      驳回
                    </button>
                  </>
                ) : null}
                {selected.status === "wait_for_approval" && usesExternalApproval ? (
                  <span className="text-xs text-muted-foreground">等待飞书/钉钉审批回调</span>
                ) : null}
                {selected.status === "executing" ? (
                  <button type="button" className="btn btn-primary text-xs" disabled={busy} onClick={() => void onExecute()}>
                    继续执行
                  </button>
                ) : null}
                {selected.status === "denied" ? (
                  <button type="button" className="btn btn-primary text-xs" disabled={busy} onClick={() => void onReopen()}>
                    重新编辑
                  </button>
                ) : null}
              </div>
              <div className="release-plan-detail-actions-secondary">
                <button type="button" className="btn-ghost text-xs" disabled={busy} onClick={() => void onCopyPlan()}>
                  复制计划
                </button>
                <button type="button" className="btn-ghost text-xs" disabled={busy} onClick={() => void onSaveTemplate()}>
                  存为模板
                </button>
                {["planning", "wait_for_approval", "executing"].includes(selected.status) ? (
                  <button type="button" className="btn-ghost text-xs text-red-600" disabled={busy} onClick={() => setCancelOpen(true)}>
                    取消计划
                  </button>
                ) : null}
              </div>
            </div>

            <SprintWorkItemsSummary
              plan={selected}
              canEdit={canEditJobs}
              busy={busy}
              onGenerateJobs={onGenerateFromWorkItems}
            />

            <section className="release-plan-detail-body">
              <div className="release-plan-detail-tabs">
                <div className="segmented-control" role="tablist" aria-label="详情视图">
                  <button
                    type="button"
                    role="tab"
                    aria-selected={detailTab === "jobs"}
                    className={`segmented-item ${detailTab === "jobs" ? "segmented-item-active" : ""}`}
                    onClick={() => setDetailTab("jobs")}
                  >
                    发布项
                    {(selected.jobs?.length ?? 0) > 0 ? ` (${selected.jobs?.length})` : ""}
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={detailTab === "audit"}
                    className={`segmented-item ${detailTab === "audit" ? "segmented-item-active" : ""}`}
                    onClick={() => setDetailTab("audit")}
                  >
                    操作记录
                  </button>
                </div>
              </div>

            {detailTab === "jobs" ? (
              <ReleasePlanJobEditor
                workspaceId={workspaceId}
                productId={productId}
                plan={selected}
                workflows={workflows}
                canEdit={canEditJobs}
                busy={busy}
                onMove={onMoveJob}
                onDelete={async (jobId) => {
                  setBusy(true);
                  try {
                    await product.deleteReleasePlanJob(workspaceId, productId, selected.id, jobId);
                    await refreshDetail();
                  } catch (err) {
                    setError(err instanceof BffError ? err.message : "删除失败");
                  } finally {
                    setBusy(false);
                  }
                }}
                onUpdate={async (jobId, patch) => {
                  const isTextOnly =
                    patch.text_content !== undefined &&
                    Object.keys(patch).every((key) => key === "text_content" || key === "text_format");
                  if (!isTextOnly) setBusy(true);
                  try {
                    await product.updateReleasePlanJob(workspaceId, productId, selected.id, jobId, patch);
                    if (!isTextOnly) await refreshDetail();
                  } catch (err) {
                    setError(err instanceof BffError ? err.message : "更新失败");
                  } finally {
                    if (!isTextOnly) setBusy(false);
                  }
                }}
                onAddText={onAddTextJob}
                onAddWorkflow={onAddWorkflowJob}
                renderJobActions={renderJobActions}
              />
            ) : (
              <ul className="release-plan-audit-list">
                {(selected.audit_logs ?? []).length === 0 ? (
                  <li className="release-plan-audit-empty">暂无操作记录</li>
                ) : (
                  (selected.audit_logs ?? []).map((log, i) => (
                    <li key={String(log.id ?? i)} className="release-plan-audit-item">
                      <span className="release-plan-audit-action">{String(log.action ?? "-")}</span>
                      {log.created_at ? (
                        <time className="release-plan-audit-time">{new Date(String(log.created_at)).toLocaleString("zh-CN")}</time>
                      ) : null}
                    </li>
                  ))
                )}
              </ul>
            )}
            </section>
          </div>
        ) : null}
      </ProductMasterDetail>

      <ReleasePlanForm
        open={createOpen}
        title="新建发布计划"
        workspaceId={workspaceId}
        busy={busy}
        onClose={() => setCreateOpen(false)}
        onSubmit={onCreate}
      />

      <ReleasePlanForm
        open={editOpen}
        title="编辑发布计划"
        workspaceId={workspaceId}
        busy={busy}
        initial={selected}
        onClose={() => setEditOpen(false)}
        onSubmit={onUpdate}
      />

      <Modal
        open={rejectOpen}
        onClose={() => setRejectOpen(false)}
        title="驳回发布计划"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => setRejectOpen(false)}
            submitLabel="确认驳回"
            submittingLabel="提交中…"
            busy={busy}
            onSubmit={() => void onReject()}
          />
        }
      >
        <label className="stack-xs text-sm">
          <span className="text-muted-foreground">驳回原因（可选）</span>
          <textarea className="input min-h-[80px]" value={rejectReason} onChange={(e) => setRejectReason(e.target.value)} />
        </label>
      </Modal>

      <Modal
        open={cancelOpen}
        onClose={() => setCancelOpen(false)}
        title="取消发布计划"
        description="取消后计划将无法继续执行，此操作不可撤销。"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => setCancelOpen(false)}
            submitLabel="确认取消"
            submittingLabel="取消中…"
            busy={busy}
            onSubmit={() => void onCancel()}
          />
        }
      >
        <p className="text-sm text-muted-foreground">确定要取消「{selected?.name}」吗？</p>
      </Modal>

      <Modal
        open={Boolean(jobDoneTarget)}
        onClose={() => {
          setJobDoneTarget(null);
          setJobDoneRemark("");
        }}
        title="标记检查项完成"
        footer={
          <ReleasePlanModalFooter
            onCancel={() => {
              setJobDoneTarget(null);
              setJobDoneRemark("");
            }}
            submitLabel="确认完成"
            submittingLabel="提交中…"
            busy={busy}
            onSubmit={() => jobDoneTarget && void onJobDone(jobDoneTarget, jobDoneRemark)}
          />
        }
      >
        <label className="stack-xs text-sm">
          <span className="text-muted-foreground">完成备注（可选）</span>
          <textarea className="input min-h-[80px]" value={jobDoneRemark} onChange={(e) => setJobDoneRemark(e.target.value)} placeholder="记录检查结果或注意事项" />
        </label>
      </Modal>
    </>
  );
}
