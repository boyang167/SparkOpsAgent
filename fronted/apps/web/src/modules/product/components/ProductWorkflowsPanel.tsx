"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  EnvironmentVM,
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
import { StartWorkflowRunModal } from "./StartWorkflowRunModal";

interface ProductWorkflowsPanelProps {
  workspaceId: string;
  productId: string;
  projects: ProductProjectVM[];
  initialWorkflowId?: string | null;
}

function statusTone(status: string): string {
  if (status === "success") return "text-green-600";
  if (status === "failed") return "text-red-600";
  if (status === "running") return "text-blue-600";
  return "text-muted-foreground";
}

export function ProductWorkflowsPanel({
  workspaceId,
  productId,
  projects,
  initialWorkflowId,
}: ProductWorkflowsPanelProps) {
  const router = useRouter();
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [templates, setTemplates] = useState<WorkflowTemplateVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(initialWorkflowId ?? null);
  const [selectedWorkflowDetail, setSelectedWorkflowDetail] = useState<ProductWorkflowVM | null>(null);
  const [runs, setRuns] = useState<ProductWorkflowRunSummaryVM[]>([]);
  const [selectedRun, setSelectedRun] = useState<ProductWorkflowRunVM | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [createMode, setCreateMode] = useState<"template" | "blank">("blank");
  const [runOpen, setRunOpen] = useState(false);
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
      const [wfItems, tplItems, envItems] = await Promise.all([
        product.listProductWorkflows(workspaceId, productId),
        product.listWorkflowTemplates(workspaceId),
        product.listEnvironments(workspaceId, productId),
      ]);
      setWorkflows(wfItems);
      setTemplates(tplItems);
      setEnvironments(envItems);
      setSelectedWorkflowId((prev) => {
        if (initialWorkflowId && wfItems.some((item) => item.id === initialWorkflowId)) return initialWorkflowId;
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
  }, [workspaceId, productId, initialWorkflowId]);

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
        setError(err instanceof BffError ? err.message : "加载执行详情失败");
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    if (!selectedWorkflowId) {
      setRuns([]);
      setSelectedRun(null);
      setSelectedWorkflowDetail(null);
      return;
    }
    void loadRuns(selectedWorkflowId);
    void product
      .getProductWorkflow(workspaceId, productId, selectedWorkflowId)
      .then(setSelectedWorkflowDetail)
      .catch(() => setSelectedWorkflowDetail(null));
  }, [selectedWorkflowId, loadRuns, workspaceId, productId]);

  useEffect(() => {
    if (!selectedRun || !selectedWorkflowId) return;
    if (selectedRun.status !== "running" && selectedRun.status !== "pending") return;
    const timer = window.setInterval(() => {
      void loadRunDetail(selectedWorkflowId, selectedRun.id);
      void loadRuns(selectedWorkflowId);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [selectedRun, selectedWorkflowId, loadRunDetail, loadRuns]);

  const selectedWorkflow = useMemo(
    () => workflows.find((wf) => wf.id === selectedWorkflowId) ?? null,
    [workflows, selectedWorkflowId],
  );

  const listItems = useMemo(
    () =>
      workflows
        .filter((wf) => !searchQuery || wf.name.toLowerCase().includes(searchQuery.toLowerCase()))
        .map((wf) => ({
          id: wf.id,
          title: wf.name,
          subtitle: `${wf.stage_count} 个阶段`,
          meta: wf.status,
        })),
    [workflows, searchQuery],
  );

  async function onCreateFromTemplate() {
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
      router.push(`/products/${productId}/workflows/${created.id}/edit`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作流失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCreateBlank() {
    setBusy(true);
    setError(null);
    try {
      const created = await product.createBlankProductWorkflow(workspaceId, productId, {
        name: name.trim() || "未命名工作流",
        activate: false,
      });
      setCreateOpen(false);
      setName("");
      router.push(`/products/${productId}/workflows/${created.id}/edit`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作流失败");
    } finally {
      setBusy(false);
    }
  }

  async function onStartRun(input: { branch?: string; environment_overrides?: Record<string, string> }) {
    if (!selectedWorkflowId) return;
    setBusy(true);
    setError(null);
    try {
      const run = await product.startProductWorkflowRun(workspaceId, productId, selectedWorkflowId, { input });
      setRunOpen(false);
      await loadRuns(selectedWorkflowId);
      setSelectedRun(run);
      router.push(`/products/${productId}/workflows/${selectedWorkflowId}/runs/${run.id}`);
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
        onAdd={() => {
          setCreateMode("blank");
          setCreateOpen(true);
        }}
        addAriaLabel="新建工作流"
        loading={loading}
        showMain={Boolean(selectedWorkflow)}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无工作流，请点击 + 新建"
            description="可空白创建并可视化编排，或从模板快速生成。"
            action={
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  className="btn btn-primary text-sm"
                  onClick={() => {
                    setCreateMode("blank");
                    setCreateOpen(true);
                  }}
                >
                  空白创建
                </button>
                <button
                  type="button"
                  className="btn btn-secondary text-sm"
                  onClick={() => {
                    setCreateMode("template");
                    setCreateOpen(true);
                  }}
                >
                  从模板创建
                </button>
              </div>
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
              <div className="flex flex-wrap gap-2">
                <Link
                  href={`/products/${productId}/workflows/${selectedWorkflow.id}/edit`}
                  className="btn btn-secondary text-sm"
                >
                  编辑编排
                </Link>
                <button
                  type="button"
                  className="btn btn-primary text-sm"
                  disabled={busy || selectedWorkflow.status !== "active"}
                  onClick={() => setRunOpen(true)}
                >
                  执行工作流
                </button>
              </div>
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
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {runs.length === 0 ? (
                      <tr>
                        <td colSpan={5} className="text-muted-foreground">
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
                          <td>
                            <Link
                              href={`/products/${productId}/workflows/${selectedWorkflow.id}/runs/${run.id}`}
                              className="text-primary text-xs"
                              onClick={(e) => e.stopPropagation()}
                            >
                              详情
                            </Link>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            {selectedRun ? (
              <div className="rounded border p-4 stack-sm text-sm">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="flex flex-wrap gap-4">
                    <span>
                      状态：<strong className={statusTone(selectedRun.status)}>{selectedRun.status}</strong>
                    </span>
                    {selectedRun.error_message ? <span className="text-red-600">{selectedRun.error_message}</span> : null}
                  </div>
                  <Link
                    href={`/products/${productId}/workflows/${selectedWorkflow.id}/runs/${selectedRun.id}`}
                    className="btn btn-ghost text-xs"
                  >
                    打开详情页
                  </Link>
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
                            {job.message ? (
                              <pre className="mt-1 whitespace-pre-wrap text-muted-foreground">{job.message}</pre>
                            ) : null}
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
                                    {d.environment_id ? ` / 环境 ${d.environment_id.slice(0, 8)}…` : null}{" "}
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
                                  </li>
                                ))}
                              </ul>
                            ) : null}
                            {job.type === "sparkops-approval" &&
                            job.approval?.status === "pending" &&
                            job.status === "running" ? (
                              <div className="ml-4 mt-1 flex flex-wrap gap-2">
                                <button
                                  type="button"
                                  className="btn-primary text-xs"
                                  disabled={busy}
                                  onClick={() => void onApproveJob(job.name)}
                                >
                                  通过
                                </button>
                                {rejectJobName === job.name ? (
                                  <>
                                    <input
                                      className="input"
                                      value={rejectMessage}
                                      onChange={(e) => setRejectMessage(e.target.value)}
                                      placeholder="驳回原因"
                                    />
                                    <button
                                      type="button"
                                      className="btn-secondary text-xs"
                                      disabled={busy}
                                      onClick={() => void onRejectJob(job.name)}
                                    >
                                      确认驳回
                                    </button>
                                  </>
                                ) : (
                                  <button
                                    type="button"
                                    className="btn-secondary text-xs"
                                    onClick={() => setRejectJobName(job.name)}
                                  >
                                    驳回
                                  </button>
                                )}
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
        title={createMode === "blank" ? "空白创建工作流" : "从模板创建工作流"}
        description={
          createMode === "blank" ? "创建草稿后进入可视化编排编辑器。" : "选择工作流模板并关联至少一个项目。"
        }
        size="lg"
        footer={
          <>
            <button
              type="button"
              className="btn btn-ghost text-sm"
              onClick={() => setCreateMode(createMode === "blank" ? "template" : "blank")}
            >
              {createMode === "blank" ? "改用模板" : "改用空白"}
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={
                busy ||
                (createMode === "template" && (!templateId || selectedProjectIds.length === 0)) ||
                (createMode === "blank" && !name.trim())
              }
              onClick={() => void (createMode === "blank" ? onCreateBlank() : onCreateFromTemplate())}
            >
              {busy ? "创建中…" : createMode === "blank" ? "创建并编辑" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-sm text-sm">
          <label className="stack-xs">
            <span className="text-muted-foreground">名称</span>
            <input
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={createMode === "blank" ? "例如：集成验证" : "覆盖模板名称（可选）"}
            />
          </label>
          {createMode === "template" ? (
            <>
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
              <div className="stack-xs">
                <span className="text-muted-foreground">关联项目（至少 1 个）</span>
                {projects.length === 0 ? (
                  <p className="text-muted-foreground">暂无关联项目，请先在项目中绑定本产品。</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {projects.map((p) => (
                      <label key={p.id} className="inline-flex items-center gap-1">
                        <input
                          type="checkbox"
                          checked={selectedProjectIds.includes(p.id)}
                          onChange={() => toggleProject(p.id)}
                        />
                        {p.display_name || p.name}
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : null}
        </div>
      </Modal>

      <StartWorkflowRunModal
        open={runOpen}
        workflow={selectedWorkflowDetail}
        environments={environments}
        busy={busy}
        onClose={() => setRunOpen(false)}
        onSubmit={(input) => void onStartRun(input)}
      />
    </>
  );
}
