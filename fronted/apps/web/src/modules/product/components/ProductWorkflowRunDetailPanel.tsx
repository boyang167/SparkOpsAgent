"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { governance, product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ProductProjectVM,
  ProductWorkflowRunSummaryVM,
  ProductWorkflowRunVM,
  ProductWorkflowVM,
} from "@sparkops/contracts/product";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  buildPipelineStages,
  formatElapsedSeconds,
  isRunActive,
  pickDefaultTask,
  runElapsedSeconds,
  type PipelineTask,
} from "./workflow-run-canvas-utils";
import { WorkflowRunCanvas } from "./WorkflowRunCanvas";
import { WorkflowRunTaskPanel } from "./WorkflowRunTaskPanel";
import {
  executorLabel,
  formatRunTime,
  runStatusLabel,
  runStatusTone,
} from "./workflow-run-history-utils";

interface ProductWorkflowRunDetailPanelProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  runId: string;
}

type DetailTab = "workflow" | "variables";

export function ProductWorkflowRunDetailPanel({
  workspaceId,
  productId,
  workflowId,
  runId,
}: ProductWorkflowRunDetailPanelProps) {
  const [workflow, setWorkflow] = useState<ProductWorkflowVM | null>(null);
  const [run, setRun] = useState<ProductWorkflowRunVM | null>(null);
  const [runSummary, setRunSummary] = useState<ProductWorkflowRunSummaryVM | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<DetailTab>("workflow");
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const [cancelOpen, setCancelOpen] = useState(false);
  const loadSeqRef = useRef(0);
  const pollBusyRef = useRef(false);

  const projectNames = useMemo(
    () =>
      Object.fromEntries(
        projects.map((item) => [item.id, item.display_name?.trim() || item.name || item.id]),
      ),
    [projects],
  );

  const memberMap = useMemo(
    () =>
      Object.fromEntries(
        members.map((item) => [item.member_id, { email: item.email, display_name: item.display_name }]),
      ),
    [members],
  );

  const pipelineStages = useMemo(
    () => (run ? buildPipelineStages(run, projectNames) : []),
    [run, projectNames],
  );

  const selectedTask = useMemo(() => {
    if (!selectedTaskId) return null;
    for (const stage of pipelineStages) {
      const found = stage.tasks.find((task) => task.id === selectedTaskId);
      if (found) return found;
    }
    return null;
  }, [pipelineStages, selectedTaskId]);

  const load = useCallback(
    async (options?: { initial?: boolean; background?: boolean }) => {
      const initial = options?.initial ?? false;
      const background = options?.background ?? false;
      if (background && pollBusyRef.current) return;

      const seq = ++loadSeqRef.current;
      if (background) pollBusyRef.current = true;
      else if (initial) setLoading(true);

      try {
        if (background) {
          const runDetail = await product.getProductWorkflowRun(workspaceId, productId, workflowId, runId);
          if (seq !== loadSeqRef.current) return;
          setRun(runDetail);
          setError(null);
          return;
        }

        const [runDetail, wf, runItems, projectItems, memberItems] = await Promise.all([
          product.getProductWorkflowRun(workspaceId, productId, workflowId, runId),
          product.getProductWorkflow(workspaceId, productId, workflowId),
          product.listProductWorkflowRuns(workspaceId, productId, workflowId),
          product.listProductProjects(workspaceId, productId),
          governance.listMembers(workspaceId).catch(() => [] as WorkspaceMemberVM[]),
        ]);
        if (seq !== loadSeqRef.current) return;

        setRun(runDetail);
        setWorkflow(wf);
        setRunSummary(runItems.find((item) => item.id === runId) ?? null);
        setProjects(projectItems);
        setMembers(memberItems);
        setError(null);
      } catch (err) {
        if (seq !== loadSeqRef.current) return;
        setError(err instanceof BffError ? err.message : "加载执行详情失败");
      } finally {
        if (background) {
          pollBusyRef.current = false;
        } else if (initial && seq === loadSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [workspaceId, productId, workflowId, runId],
  );

  useEffect(() => {
    setLoading(true);
    setRun(null);
    setWorkflow(null);
    setRunSummary(null);
    void load({ initial: true });
    return () => {
      loadSeqRef.current += 1;
    };
  }, [load]);

  useEffect(() => {
    if (!run || !isRunActive(run.status)) return;
    const timer = window.setInterval(() => {
      void load({ background: true });
    }, 2500);
    return () => window.clearInterval(timer);
  }, [run, load]);

  useEffect(() => {
    if (!run || !isRunActive(run.status)) return;
    const timer = window.setInterval(() => setTick((value) => value + 1), 1000);
    return () => window.clearInterval(timer);
  }, [run]);

  useEffect(() => {
    if (!run || selectedTaskId) return;
    const stages = buildPipelineStages(run, projectNames);
    const next = pickDefaultTask(stages);
    if (next) setSelectedTaskId(next.id);
  }, [run, projectNames, selectedTaskId]);

  async function onPause() {
    setBusy(true);
    setError(null);
    try {
      await product.pauseProductWorkflowRun(workspaceId, productId, workflowId, runId);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "暂停任务失败");
    } finally {
      setBusy(false);
    }
  }

  async function onResume() {
    setBusy(true);
    setError(null);
    try {
      await product.continueProductWorkflowRun(workspaceId, productId, workflowId, runId);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "继续执行失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCancel() {
    setBusy(true);
    setError(null);
    try {
      await product.cancelProductWorkflowRun(workspaceId, productId, workflowId, runId, {
        message: "用户取消任务",
      });
      setCancelOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "取消任务失败");
    } finally {
      setBusy(false);
    }
  }

  const elapsed = run ? formatElapsedSeconds(runElapsedSeconds(run)) : "—";
  void tick;

  if (loading) {
    return <div className="empty-state text-sm" role="status">加载执行详情…</div>;
  }

  if (!run || !workflow) {
    return (
      <div className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="empty-state text-sm">执行记录不存在或无权访问。</div>
      </div>
    );
  }

  const runNumber = runSummary?.run_number;
  const executor = runSummary ? executorLabel(runSummary, memberMap) : "—";
  const isUserPaused = run.status === "paused" && run.pause_kind === "user";
  const isManualStagePaused = run.status === "paused" && !isUserPaused;

  return (
    <div className="wf-run-live-page stack-lg">
      <header className="wf-run-live-header">
        <div className="wf-run-live-header-main">
          <div className="wf-run-live-title-row">
            <h1 className="wf-run-live-title">
              {workflow.name}
              {runNumber != null ? <span className="wf-run-live-run-no">#{runNumber}</span> : null}
            </h1>
            <span className={`wf-run-live-status ${runStatusTone(run.status)}`}>{runStatusLabel(run.status)}</span>
          </div>
          <div className="wf-run-live-meta-row">
            <span>{formatRunTime(run.started_at ?? run.created_at)}</span>
            <span className="wf-run-live-meta-sep">·</span>
            <span>{elapsed}</span>
            <span className="wf-run-live-meta-sep">·</span>
            <span>{executor}</span>
          </div>
        </div>
        <div className="wf-run-live-header-actions">
          <Link href={`/products/${productId}/workflows/${workflowId}`} className="btn btn-ghost text-sm">
            返回执行记录
          </Link>
          {run.status === "running" ? (
            <button type="button" className="btn btn-ghost text-sm" disabled={busy} onClick={() => void onPause()}>
              {busy ? "暂停中…" : "暂停任务"}
            </button>
          ) : null}
          {isRunActive(run.status) ? (
            <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={() => setCancelOpen(true)}>
              取消任务
            </button>
          ) : null}
        </div>
      </header>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {run.error_message ? <InlineAlert tone="error">{run.error_message}</InlineAlert> : null}

      {run.status === "paused" ? (
        <div className="wf-run-live-paused">
          <InlineAlert tone="info">
            {isUserPaused
              ? "任务已暂停。进行中的构建/部署会继续完成，但不会进入下一阶段，点击继续执行恢复调度。"
              : isManualStagePaused
                ? "当前阶段为手动执行，确认后才会进入该阶段任务。"
                : "任务已暂停，点击继续执行恢复调度。"}
          </InlineAlert>
          <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onResume()}>
            {busy ? "恢复中…" : "继续执行"}
          </button>
        </div>
      ) : null}

      <div className="wf-run-live-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "workflow"}
          className={tab === "workflow" ? "is-active" : undefined}
          onClick={() => setTab("workflow")}
        >
          工作流
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "variables"}
          className={tab === "variables" ? "is-active" : undefined}
          onClick={() => setTab("variables")}
        >
          变量
        </button>
      </div>

      {tab === "workflow" ? (
        <div className="wf-run-live-body">
          <WorkflowRunCanvas
            stages={pipelineStages}
            selectedTaskId={selectedTaskId}
            onSelectTask={(task: PipelineTask) => setSelectedTaskId(task.id)}
          />
          <WorkflowRunTaskPanel
            workspaceId={workspaceId}
            productId={productId}
            workflowId={workflowId}
            runId={runId}
            task={selectedTask}
            onClose={() => setSelectedTaskId(null)}
            onRefresh={() => void load()}
          />
        </div>
      ) : (
        <section className="wf-run-live-vars card stack-sm">
          {workflow.variables?.length ? (
            <div className="data-table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>键</th>
                    <th>说明</th>
                    <th>运行时输入</th>
                  </tr>
                </thead>
                <tbody>
                  {workflow.variables.map((item) => (
                    <tr key={item.id ?? item.key}>
                      <td>{item.key}</td>
                      <td>{item.description || "—"}</td>
                      <td>{item.runtime_input ? "是" : "否"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="empty-state text-sm">该工作流未配置变量。</div>
          )}
        </section>
      )}

      {run.remark ? (
        <div className="pwf-run-remark">
          <span className="pwf-run-remark-label">备注</span>
          <p className="pwf-run-remark-body">{run.remark}</p>
        </div>
      ) : null}

      <Modal
        open={cancelOpen}
        title="取消任务"
        onClose={() => setCancelOpen(false)}
        footer={
          <>
            <button type="button" className="btn btn-ghost text-sm" disabled={busy} onClick={() => setCancelOpen(false)}>
              返回
            </button>
            <button type="button" className="btn btn-secondary text-sm" disabled={busy} onClick={() => void onCancel()}>
              {busy ? "取消中…" : "确认取消"}
            </button>
          </>
        }
      >
        <p className="text-sm text-secondary">取消后当前执行任务将标记为失败并停止后续阶段，此操作不可撤销。</p>
      </Modal>
    </div>
  );
}
