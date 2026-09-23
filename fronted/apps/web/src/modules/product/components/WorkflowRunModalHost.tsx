"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  EnvironmentVM,
  ProductProjectVM,
  ProductWorkflowVM,
  StartProductWorkflowRunCommand,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { StartWorkflowRunModal } from "./StartWorkflowRunModal";
import {
  peekProductEnvironments,
  prefetchProductEnvironments,
  prefetchWorkspaceRunAssets,
} from "./workflow-run-prefetch";

export interface WorkflowRunTarget {
  productId: string;
  workflowId: string;
}

interface WorkflowRunModalHostProps {
  workspaceId: string;
  target: WorkflowRunTarget | null;
  open: boolean;
  seedWorkflow?: ProductWorkflowVM | null;
  seedProjects?: ProductProjectVM[];
  seedEnvironments?: EnvironmentVM[];
  workflowName?: string;
  onClose: () => void;
  onStarted?: () => void;
}

export function WorkflowRunModalHost({
  workspaceId,
  target,
  open,
  seedWorkflow = null,
  seedProjects = [],
  seedEnvironments,
  workflowName,
  onClose,
  onStarted,
}: WorkflowRunModalHostProps) {
  const router = useRouter();
  const [workflow, setWorkflow] = useState<ProductWorkflowVM | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const applySeedData = useCallback(() => {
    if (!target) return false;
    const cachedEnvironments =
      seedEnvironments ?? peekProductEnvironments(workspaceId, target.productId) ?? [];
    if (seedWorkflow?.status === "active") {
      setWorkflow(seedWorkflow);
      setProjects(seedProjects);
      setEnvironments(cachedEnvironments);
      setLoading(false);
      setError(null);
      if (cachedEnvironments.length === 0) {
        void prefetchProductEnvironments(workspaceId, target.productId).then((items) => {
          setEnvironments(items);
        });
      }
      return true;
    }
    return false;
  }, [seedEnvironments, seedProjects, seedWorkflow, target, workspaceId]);

  const load = useCallback(async () => {
    if (!target) return;
    setLoading(true);
    setError(null);
    try {
      const cachedEnvironments = peekProductEnvironments(workspaceId, target.productId);
      const envPromise =
        cachedEnvironments != null
          ? Promise.resolve(cachedEnvironments)
          : prefetchProductEnvironments(workspaceId, target.productId);
      const [wf, projectItems, envItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, target.productId, target.workflowId),
        seedProjects.length > 0
          ? Promise.resolve(seedProjects)
          : product.listProductProjects(workspaceId, target.productId),
        envPromise,
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
  }, [seedProjects, target, workspaceId]);

  useEffect(() => {
    if (!open || !target) {
      setWorkflow(null);
      setProjects([]);
      setEnvironments([]);
      setError(null);
      setBusy(false);
      setLoading(false);
      return;
    }

    prefetchWorkspaceRunAssets(workspaceId);

    if (applySeedData()) {
      return;
    }

    void load();
  }, [open, target, applySeedData, load, workspaceId]);

  async function onStartRun(input: StartProductWorkflowRunCommand["input"]) {
    if (!target || !workflow) return;
    setBusy(true);
    setError(null);
    try {
      const run = await product.startProductWorkflowRun(workspaceId, target.productId, target.workflowId, { input });
      onClose();
      onStarted?.();
      router.push(`/products/${target.productId}/workflows/${target.workflowId}/runs/${run.id}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动执行失败");
    } finally {
      setBusy(false);
    }
  }

  if (!open || !target) return null;

  if (!loading && (!workflow || workflow.status !== "active")) {
    return (
      <Modal
        open
        onClose={onClose}
        title="执行工作流"
        size="sm"
        footer={
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            关闭
          </button>
        }
      >
        <div className="stack-sm">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          <div className="empty-state text-sm">该工作流暂不可执行。</div>
        </div>
      </Modal>
    );
  }

  return (
    <StartWorkflowRunModal
      open={open}
      workspaceId={workspaceId}
      workflow={workflow}
      environments={environments}
      projects={projects}
      busy={busy}
      loading={loading || !workflow}
      workflowName={workflow?.name ?? workflowName}
      submitError={error}
      onClose={onClose}
      onSubmit={(input) => void onStartRun(input)}
    />
  );
}
