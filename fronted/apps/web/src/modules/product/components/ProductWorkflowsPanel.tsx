"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ProductProjectVM,
  ProductWorkflowRunSummaryVM,
  ProductWorkflowRunVM,
  ProductWorkflowVM,
  WorkflowTemplateVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";

interface ProductWorkflowsPanelProps {
  workspaceId: string;
  productId: string;
  projects: ProductProjectVM[];
}

function statusTone(status: string): string {
  if (status === "success") return "text-green-600";
  if (status === "failed") return "text-red-600";
  if (status === "running") return "text-blue-600";
  return "text-muted-foreground";
}

export function ProductWorkflowsPanel({ workspaceId, productId, projects }: ProductWorkflowsPanelProps) {
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [templates, setTemplates] = useState<WorkflowTemplateVM[]>([]);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(null);
  const [runs, setRuns] = useState<ProductWorkflowRunSummaryVM[]>([]);
  const [selectedRun, setSelectedRun] = useState<ProductWorkflowRunVM | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [templateId, setTemplateId] = useState("");
  const [name, setName] = useState("");
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [rejectJobName, setRejectJobName] = useState<string | null>(null);
  const [rejectMessage, setRejectMessage] = useState("");

  const loadWorkflows = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [wfItems, tplItems] = await Promise.all([
        product.listProductWorkflows(workspaceId, productId),
        product.listWorkflowTemplates(workspaceId),
      ]);
      setWorkflows(wfItems);
      setTemplates(tplItems);
      setSelectedWorkflowId((prev) => {
        if (prev && wfItems.some((item) => item.id === prev)) return prev;
        return wfItems[0]?.id ?? null;
      });
      if (tplItems.length > 0) {
        setTemplateId((prev) => prev || tplItems[0].id);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载产品工作流失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void loadWorkflows();
  }, [loadWorkflows]);

  const loadRuns = useCallback(
    async (workflowId: string) => {
      try {
        setRuns(await product.listProductWorkflowRuns(workspaceId, productId, workflowId));
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载执行记录失败");
      }
    },
    [workspaceId, productId],
  );

  const loadRunDetail = useCallback(
    async (workflowId: string, runId: string) => {
      try {
        setSelectedRun(await product.getProductWorkflowRun(workspaceId, productId, workflowId, runId));
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载 Run 详情失败");
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    if (!selectedWorkflowId) {
      setRuns([]);
      setSelectedRun(null);
      return;
    }
    setSelectedRun(null);
    void loadRuns(selectedWorkflowId);
  }, [selectedWorkflowId, loadRuns]);

  const runNeedsPolling =
    selectedRun?.status === "running" ||
    selectedRun?.status === "pending" ||
    selectedRun?.stage_runs.some((stage) =>
      stage.jobs.some(
        (job) =>
          job.status === "running" ||
          job.status === "pending" ||
          (job.type === "sparkops-approval" && job.approval?.status === "pending"),
      ),
    );

  useEffect(() => {
    if (!selectedWorkflowId || !selectedRun || !runNeedsPolling) return;
    const timer = window.setInterval(() => {
      void loadRunDetail(selectedWorkflowId, selectedRun.id);
      void loadRuns(selectedWorkflowId);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [selectedWorkflowId, selectedRun, runNeedsPolling, loadRunDetail, loadRuns]);

  const selectedWorkflow = useMemo(
    () => workflows.find((wf) => wf.id === selectedWorkflowId) ?? null,
    [workflows, selectedWorkflowId],
  );

  const listItems = useMemo(
    () =>
      workflows.map((wf) => ({
        id: wf.id,
        title: wf.name,
        subtitle: `${wf.stage_count} 个阶段`,
        meta: wf.status,
      })),
    [workflows],
  );

  async function onCreate() {
    if (!templateId || selectedProjectIds.length === 0) return;
    setBusy(true);
    setError(null);
    try {
      const created = await product.createProductWorkflowFromTemplate(workspaceId, productId, {
        template_id: templateId,
        name: name.trim() || undefined,
        project_ids: selectedProjectIds,
      });
      setCreateOpen(false);
      setName("");
      setSelectedProjectIds([]);
      await loadWorkflows();
      setSelectedWorkflowId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作流失败");
    } finally {
      setBusy(false);
    }
  }

  async function onStartRun() {
    if (!selectedWorkflowId) return;
    setBusy(true);
    setError(null);
    try {
      const run = await product.startProductWorkflowRun(workspaceId, productId, selectedWorkflowId);
      await loadRuns(selectedWorkflowId);
      setSelectedRun(run);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动执行失败");
    } finally {
      setBusy(false);
    }
  }

  async function onApproveJob(jobName: string) {
    if (!selectedWorkflowId || !selectedRun) return;
    setBusy(true);
    setError(null);
    try {
      await product.approveWorkflowJob(workspaceId, productId, selectedWorkflowId, selectedRun.id, jobName);
      await loadRunDetail(selectedWorkflowId, selectedRun.id);
      await loadRuns(selectedWorkflowId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "审批通过失败");
    } finally {
      setBusy(false);
    }
  }

  async function onRejectJob(jobName: string) {
    if (!selectedWorkflowId || !selectedRun) return;
    setBusy(true);
    setError(null);
    try {
      await product.rejectWorkflowJob(workspaceId, productId, selectedWorkflowId, selectedRun.id, jobName, {
        message: rejectMessage.trim() || undefined,
      });
      setRejectJobName(null);
      setRejectMessage("");
      await loadRunDetail(selectedWorkflowId, selectedRun.id);
      await loadRuns(selectedWorkflowId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "审批驳回失败");
    } finally {
      setBusy(false);
    }
  }

  function toggleProject(projectId: string) {
    setSelectedProjectIds((prev) =>
      prev.includes(projectId) ? prev.filter((id) => id !== projectId) : [...prev, projectId],
    );
  }

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="工作流"
        searchPlaceholder="搜索工作流"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedWorkflowId}
        onSelect={(id) => {
          setSelectedWorkflowId(id);
          setSelectedRun(null);
        }}
        onAdd={() => setCreateOpen(true)}
        addAriaLabel="从模板创建工作流"
        loading={loading}
        showMain={Boolean(selectedWorkflow)}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无工作流，请点击 + 新建"
            description="从模板创建产品工作流，编排跨项目的构建与部署。"
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateOpen(true)}>
                从模板创建
              </button>
            }
          />
        }
      >
        {selectedWorkflow ? (
          <div className="product-master-main-inner stack-md">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="stack-xs">
                <h2 className="section-title">{selectedWorkflow.name}</h2>
                <p className="text-sm text-muted-foreground">
                  {selectedWorkflow.stage_count} 个阶段 · {selectedWorkflow.status}
                </p>
              </div>
              <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onStartRun()}>
                执行工作流
              </button>
            </div>

            <div className="stack-sm">
              <h3 className="text-sm font-medium">执行记录</h3>
              <div className="table-wrap">
                <table className="data-table text-sm">
                  <thead>
                    <tr>
                      <th>Run ID</th>
                      <th>状态</th>
                      <th>当前阶段</th>
                      <th>创建时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runs.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="text-muted-foreground">
                          暂无执行记录
                        </td>
                      </tr>
                    ) : (
                      runs.map((run) => (
                        <tr
                          key={run.id}
                          className={selectedRun?.id === run.id ? "bg-muted/40 cursor-pointer" : "cursor-pointer"}
                          onClick={() => void loadRunDetail(selectedWorkflowId!, run.id)}
                        >
                          <td className="font-mono text-xs">{run.id.slice(0, 8)}…</td>
                          <td className={statusTone(run.status)}>{run.status}</td>
                          <td>{run.current_stage}</td>
                          <td>{run.created_at ? new Date(run.created_at).toLocaleString() : "-"}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            {selectedRun ? (
              <div className="rounded border p-4 stack-sm text-sm">
                <div className="flex flex-wrap gap-4">
                  <span>
                    状态：<strong className={statusTone(selectedRun.status)}>{selectedRun.status}</strong>
                  </span>
                  {selectedRun.error_message ? <span className="text-red-600">{selectedRun.error_message}</span> : null}
                </div>
                <div className="stack-sm">
                  {selectedRun.stage_runs.map((stage, si) => (
                    <div key={`${stage.name}-${si}`} className="border-l-2 pl-3 border-muted">
                      <div className="font-medium">
                        阶段 {si + 1}：{stage.name}{" "}
                        <span className={statusTone(stage.status)}>({stage.status})</span>
                      </div>
                      <ul className="mt-1 stack-xs text-xs">
                        {stage.jobs.map((job, ji) => (
                          <li key={`${job.name}-${ji}`}>
                            <span className="font-medium">{job.name}</span>{" "}
                            <span className="text-muted-foreground">[{job.type}]</span>{" "}
                            <span className={statusTone(job.status)}>{job.status}</span>
                            {job.error_message ? <span className="text-red-600 ml-1">{job.error_message}</span> : null}
                            {job.builds && job.builds.length > 0 ? (
                              <ul className="ml-4 mt-1">
                                {job.builds.map((b) => (
                                  <li key={`${b.project_id}-${b.branch}`}>
                                    项目 {b.project_id.slice(0, 8)}… / {b.branch ?? "main"}{" "}
                                    <span className={statusTone(b.status)}>{b.status}</span>
                                    {b.build_id ? (
                                      <>
                                        {" "}
                                        <Link href={`/delivery/builds/${b.build_id}`} className="text-primary hover:underline">
                                          查看构建
                                        </Link>
                                      </>
                                    ) : null}
                                  </li>
                                ))}
                              </ul>
                            ) : null}
                            {job.deploys && job.deploys.length > 0 ? (
                              <ul className="ml-4 mt-1">
                                {job.deploys.map((d) => (
                                  <li key={`${d.project_id}-${d.environment_id ?? "env"}`}>
                                    项目 {d.project_id.slice(0, 8)}…
                                    {d.environment_id ? ` / 环境 ${d.environment_id.slice(0, 8)}…` : null}
                                    {d.image_tag ? ` / ${d.image_tag}` : null}{" "}
                                    <span className={statusTone(d.status)}>{d.status}</span>
                                    {d.deployment_run_id ? (
                                      <>
                                        {" "}
                                        <Link
                                          href={`/delivery/deployments/${d.deployment_run_id}`}
                                          className="text-primary hover:underline"
                                        >
                                          查看部署
                                        </Link>
                                      </>
                                    ) : null}
                                    {d.error_message ? <span className="text-red-600 ml-1">{d.error_message}</span> : null}
                                  </li>
                                ))}
                              </ul>
                            ) : null}
                            {job.type === "sparkops-approval" && job.approval ? (
                              <div className="ml-4 mt-1 stack-xs">
                                <span>
                                  审批状态：
                                  <span
                                    className={statusTone(
                                      job.approval.status === "approved"
                                        ? "success"
                                        : job.approval.status === "rejected"
                                          ? "failed"
                                          : "running",
                                    )}
                                  >
                                    {job.approval.status}
                                  </span>
                                  {job.approval.message ? (
                                    <span className="text-muted-foreground ml-1">({job.approval.message})</span>
                                  ) : null}
                                </span>
                                {job.approval.status === "pending" && job.status === "running" ? (
                                  rejectJobName === job.name ? (
                                    <div className="flex flex-wrap items-end gap-2">
                                      <label className="stack-xs">
                                        <span className="text-muted-foreground">驳回原因（可选）</span>
                                        <input
                                          className="input"
                                          value={rejectMessage}
                                          onChange={(e) => setRejectMessage(e.target.value)}
                                          placeholder="填写驳回说明"
                                        />
                                      </label>
                                      <button
                                        type="button"
                                        className="btn-secondary text-xs"
                                        disabled={busy}
                                        onClick={() => void onRejectJob(job.name)}
                                      >
                                        确认驳回
                                      </button>
                                      <button
                                        type="button"
                                        className="btn-secondary text-xs"
                                        disabled={busy}
                                        onClick={() => {
                                          setRejectJobName(null);
                                          setRejectMessage("");
                                        }}
                                      >
                                        取消
                                      </button>
                                    </div>
                                  ) : (
                                    <div className="flex flex-wrap gap-2">
                                      <button
                                        type="button"
                                        className="btn-primary text-xs"
                                        disabled={busy}
                                        onClick={() => void onApproveJob(job.name)}
                                      >
                                        通过
                                      </button>
                                      <button
                                        type="button"
                                        className="btn-secondary text-xs"
                                        disabled={busy}
                                        onClick={() => setRejectJobName(job.name)}
                                      >
                                        驳回
                                      </button>
                                    </div>
                                  )
                                ) : null}
                              </div>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        ) : null}
      </ProductMasterDetail>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="从模板创建工作流"
        description="选择工作流模板并关联至少一个项目。"
        size="lg"
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || !templateId || selectedProjectIds.length === 0}
              onClick={() => void onCreate()}
            >
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-sm text-sm">
          <label className="stack-xs">
            <span className="text-muted-foreground">工作流模板</span>
            <select className="input" value={templateId} onChange={(e) => setTemplateId(e.target.value)}>
              {templates.map((tpl) => (
                <option key={tpl.id} value={tpl.id}>
                  {tpl.name} ({tpl.key})
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">名称（可选）</span>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="覆盖模板名称" />
          </label>
          <div className="stack-xs">
            <span className="text-muted-foreground">关联项目（至少 1 个）</span>
            {projects.length === 0 ? (
              <p className="text-muted-foreground">暂无关联项目，请先在项目中绑定本产品。</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {projects.map((p) => (
                  <label key={p.id} className="inline-flex items-center gap-1">
                    <input type="checkbox" checked={selectedProjectIds.includes(p.id)} onChange={() => toggleProject(p.id)} />
                    {p.display_name || p.name}
                  </label>
                ))}
              </div>
            )}
          </div>
        </div>
      </Modal>
    </>
  );
}
