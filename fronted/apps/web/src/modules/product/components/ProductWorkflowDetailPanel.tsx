"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { governance, product, workspace } from "@sparkops/web-sdk";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import type { ProjectVM } from "@sparkops/contracts/workspace";
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
  const [workspaceProjects, setWorkspaceProjects] = useState<ProjectVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [runOpen, setRunOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [wf, runItems, projectItems, envItems, memberItems, workspaceProjectItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, productId, workflowId),
        product.listProductWorkflowRuns(workspaceId, productId, workflowId),
        product.listProductProjects(workspaceId, productId),
        product.listEnvironments(workspaceId, productId),
        governance.listMembers(workspaceId).catch(() => [] as WorkspaceMemberVM[]),
        workspace.listProjects(workspaceId).catch(() => [] as ProjectVM[]),
      ]);
      setWorkflow(wf);
      setRuns(runItems);
      setProjects(projectItems);
      setEnvironments(envItems);
      setMembers(memberItems);
      setWorkspaceProjects(workspaceProjectItems);
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
        [...workspaceProjects, ...projects].map((item) => [item.id, item.display_name?.trim() || item.name]),
      ),
    [workspaceProjects, projects],
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
        <nav className="wf-detail-breadcrumb" aria-label="面包屑">
          <Link href="/products/workflows" className="wf-detail-back">
            工作流
          </Link>
          <span aria-hidden>/</span>
          <span>{workflow.name}</span>
        </nav>
        <div className="wf-detail-title-row">
          <div className="wf-detail-title-main">
            <h1 className="wf-detail-title">{workflow.name}</h1>
            {workflow.status !== "active" ? <span className="wf-list-badge">草稿</span> : null}
          </div>
          <div className="wf-detail-header-actions">
            <Link
              href={`/products/${productId}/workflows/${workflowId}/edit`}
              className="wf-detail-icon-btn"
              title="编排设置"
              aria-label="编排设置"
            >
              <IconSettings />
            </Link>
            <button
              type="button"
              className="btn btn-primary text-sm wf-detail-run-btn"
              disabled={busy || workflow.status !== "active"}
              title={workflow.status !== "active" ? "草稿状态的工作流需先发布才能执行" : undefined}
              onMouseEnter={() => prefetchWorkspaceRunAssets(workspaceId)}
              onFocus={() => prefetchWorkspaceRunAssets(workspaceId)}
              onClick={() => setRunOpen(true)}
            >
              <IconPlay />
              执行
            </button>
          </div>
        </div>
        {workflow.description ? <p className="wf-detail-desc">{workflow.description}</p> : null}
        <div className="wf-detail-meta">
          <WorkflowStagePath
            labels={stageLabels}
            fallback={workflowStagePathText(workflow, workflow.stage_count)}
          />
          <span className="wf-detail-meta-sep" aria-hidden>·</span>
          <span>最后修改 {formatLastRun(workflow.updated_at ?? null)}</span>
        </div>
      </header>

      {error && !runOpen ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <section className="wf-history-section">
        <header className="wf-history-head">
          <h2 className="wf-history-title">执行记录</h2>
          {runs.length > 0 ? <span className="wf-history-count">{runs.length}</span> : null}
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
