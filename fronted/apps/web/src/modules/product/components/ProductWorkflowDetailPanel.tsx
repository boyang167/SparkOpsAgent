"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { governance, product } from "@sparkops/web-sdk";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { BffError } from "@sparkops/contracts";
import type {
  EnvironmentVM,
  ProductProjectVM,
  ProductWorkflowRunSummaryVM,
  ProductWorkflowVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { StartWorkflowRunModal } from "./StartWorkflowRunModal";
import { WorkflowRunHistoryTable } from "./WorkflowRunHistoryTable";
import { IconPlay, IconSettings } from "./WorkflowListIcons";
import { formatLastRun, stagePathLabels, workflowStagePathText } from "./workflow-list-utils";
import { WorkflowStagePath } from "./WorkflowStagePath";
import { prefetchWorkspaceRunAssets } from "./workflow-run-prefetch";

interface ProductWorkflowDetailPanelProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
}

export function ProductWorkflowDetailPanel({
  workspaceId,
  productId,
  workflowId,
}: ProductWorkflowDetailPanelProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [workflow, setWorkflow] = useState<ProductWorkflowVM | null>(null);
  const [runs, setRuns] = useState<ProductWorkflowRunSummaryVM[]>([]);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [runOpen, setRunOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [wf, runItems, projectItems, envItems, memberItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, productId, workflowId),
        product.listProductWorkflowRuns(workspaceId, productId, workflowId),
        product.listProductProjects(workspaceId, productId),
        product.listEnvironments(workspaceId, productId),
        governance.listMembers(workspaceId).catch(() => [] as WorkspaceMemberVM[]),
      ]);
      setWorkflow(wf);
      setRuns(runItems);
      setProjects(projectItems);
      setEnvironments(envItems);
      setMembers(memberItems);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作流失败");
      setWorkflow(null);
      setRuns([]);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId, workflowId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    prefetchWorkspaceRunAssets(workspaceId);
  }, [workspaceId]);

  useEffect(() => {
    if (searchParams.get("run") === "1" && workflow?.status === "active" && !loading) {
      setRunOpen(true);
    }
  }, [searchParams, workflow?.status, loading]);

  const stageLabels = useMemo(() => stagePathLabels(workflow?.stages), [workflow?.stages]);
  const projectNames = useMemo(
    () =>
      Object.fromEntries(
        projects.map((item) => [item.id, item.display_name?.trim() || item.name || item.id]),
      ),
    [projects],
  );
  const memberNames = useMemo(
    () =>
      Object.fromEntries(
        members.map((item) => [item.member_id, { email: item.email, display_name: item.display_name }]),
      ),
    [members],
  );

  async function onStartRun(input: import("@sparkops/contracts/product").StartProductWorkflowRunCommand["input"]) {
    if (!workflow) return;
    setBusy(true);
    setError(null);
    try {
      const run = await product.startProductWorkflowRun(workspaceId, productId, workflowId, { input });
      setRunOpen(false);
      router.push(`/products/${productId}/workflows/${workflowId}/runs/${run.id}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动执行失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return <div className="empty-state text-sm">加载工作流…</div>;
  }

  if (!workflow) {
    return (
      <div className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="empty-state text-sm">工作流不存在或无权访问。</div>
        <Link href="/products/workflows" className="btn btn-ghost text-sm">
          返回工作流列表
        </Link>
      </div>
    );
  }

  return (
    <div className="stack-lg wf-detail-page">
      <header className="wf-detail-header">
        <div className="wf-detail-header-main">
          <div className="wf-detail-breadcrumb">
            <Link href="/products/workflows" className="wf-detail-back">
              工作流
            </Link>
            <span aria-hidden>/</span>
            <span>{workflow.name}</span>
          </div>
          <div className="wf-detail-title-row">
            <h1 className="wf-detail-title">{workflow.name}</h1>
            {workflow.status !== "active" ? <span className="wf-list-badge">草稿</span> : null}
          </div>
          <div className="wf-detail-stage-row">
            <WorkflowStagePath
              labels={stageLabels}
              fallback={workflowStagePathText(workflow, workflow.stage_count)}
            />
          </div>
          <div className="wf-detail-meta text-sm text-muted">
            <span>最后修改：{formatLastRun(workflow.updated_at ?? null)}</span>
            {workflow.description ? <span>{workflow.description}</span> : null}
          </div>
        </div>
        <div className="wf-detail-header-actions">
          <button
            type="button"
            className="wf-list-run-btn"
            disabled={busy || workflow.status !== "active"}
            onMouseEnter={() => prefetchWorkspaceRunAssets(workspaceId)}
            onFocus={() => prefetchWorkspaceRunAssets(workspaceId)}
            onClick={() => setRunOpen(true)}
          >
            <IconPlay />
            执行
          </button>
          <Link
            href={`/products/${productId}/workflows/${workflowId}/edit`}
            className="wf-list-icon-btn wf-list-settings-btn"
            title="编排设置"
          >
            <IconSettings />
          </Link>
        </div>
      </header>

      {error && !runOpen ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <section className="wf-history-section">
        <header className="wf-history-head">
          <h2 className="wf-history-title">执行记录</h2>
          <span className="wf-history-count">{runs.length} 条</span>
        </header>
        <WorkflowRunHistoryTable
          workspaceId={workspaceId}
          productId={productId}
          workflowId={workflowId}
          runs={runs}
          projectNames={projectNames}
          memberNames={memberNames}
          onUpdated={() => void load()}
        />
      </section>

      <StartWorkflowRunModal
        open={runOpen}
        workspaceId={workspaceId}
        workflow={workflow}
        environments={environments}
        projects={projects}
        busy={busy}
        submitError={runOpen ? error : null}
        onClose={() => {
          setRunOpen(false);
          setError(null);
        }}
        onSubmit={(input) => void onStartRun(input)}
      />
    </div>
  );
}
