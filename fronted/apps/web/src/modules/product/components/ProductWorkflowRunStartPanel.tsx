"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentVM, ProductProjectVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { StartWorkflowRunModal } from "./StartWorkflowRunModal";

interface ProductWorkflowRunStartPanelProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
}

export function ProductWorkflowRunStartPanel({
  workspaceId,
  productId,
  workflowId,
}: ProductWorkflowRunStartPanelProps) {
  const router = useRouter();
  const detailHref = `/products/${productId}/workflows/${workflowId}`;
  const [workflow, setWorkflow] = useState<ProductWorkflowVM | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [wf, projectItems, envItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, productId, workflowId),
        product.listProductProjects(workspaceId, productId),
        product.listEnvironments(workspaceId, productId),
      ]);
      if (wf.status !== "active") {
        setError("草稿工作流无法执行，请先保存并启用。");
        setWorkflow(wf);
        return;
      }
      setWorkflow(wf);
      setProjects(projectItems);
      setEnvironments(envItems);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作流失败");
      setWorkflow(null);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId, workflowId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onStartRun(input: import("@sparkops/contracts/product").StartProductWorkflowRunCommand["input"]) {
    if (!workflow) return;
    setBusy(true);
    setError(null);
    try {
      const run = await product.startProductWorkflowRun(workspaceId, productId, workflowId, { input });
      router.push(`/products/${productId}/workflows/${workflowId}/runs/${run.id}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动执行失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return <div className="empty-state text-sm" role="status">加载执行配置…</div>;
  }

  if (!workflow) {
    return (
      <div className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="empty-state text-sm">工作流不存在或无权访问。</div>
        <Link href={detailHref} className="btn btn-ghost text-sm">
          返回工作流
        </Link>
      </div>
    );
  }

  if (workflow.status !== "active") {
    return (
      <div className="stack-md wf-run-page">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="empty-state text-sm">草稿工作流无法执行，请先进入编排保存并启用。</div>
        <div className="flex flex-wrap gap-2">
          <Link href={`${detailHref}/edit`} className="btn btn-primary text-sm">
            进入编排
          </Link>
          <Link href={detailHref} className="btn btn-ghost text-sm">
            返回工作流
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="wf-run-page-shell">
      <nav className="wf-run-page-nav" aria-label="面包屑">
        <Link href="/products/workflows">工作流</Link>
        <span aria-hidden>/</span>
        <Link href={detailHref}>{workflow.name}</Link>
        <span aria-hidden>/</span>
        <span>执行</span>
      </nav>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <StartWorkflowRunModal
        presentation="page"
        workspaceId={workspaceId}
        workflow={workflow}
        environments={environments}
        projects={projects}
        busy={busy}
        onClose={() => router.push(detailHref)}
        onSubmit={(input) => void onStartRun(input)}
      />
    </div>
  );
}
