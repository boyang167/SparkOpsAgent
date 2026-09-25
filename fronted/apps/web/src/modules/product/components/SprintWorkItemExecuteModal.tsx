"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  EnvironmentVM,
  ProductProjectVM,
  ProductWorkflowVM,
  SprintStage,
  StartProductWorkflowRunCommand,
  WorkItemCardVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { StartWorkflowRunModal } from "./StartWorkflowRunModal";
import { WorkflowRunFormSkeleton } from "./WorkflowRunFormSkeleton";
import {
  peekProductEnvironments,
  prefetchProductEnvironments,
  prefetchWorkspaceRunAssets,
} from "./workflow-run-prefetch";

interface SprintWorkItemExecuteModalProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  productId: string;
  stage: SprintStage;
  workflowId: string;
  workItems: WorkItemCardVM[];
  initialSelectedId: string;
  onExecuted: () => void;
}

export function SprintWorkItemExecuteModal({
  open,
  onClose,
  workspaceId,
  productId,
  stage,
  workflowId,
  workItems,
  initialSelectedId,
  onExecuted,
}: SprintWorkItemExecuteModalProps) {
  const [workflow, setWorkflow] = useState<ProductWorkflowVM | null>(null);
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());

  const stageWorkflows = useMemo(
    () => stage.workflows?.filter((binding) => binding.workflow_id) ?? [],
    [stage.workflows],
  );
  const [activeWorkflowId, setActiveWorkflowId] = useState(workflowId);

  const selectableItems = useMemo(
    () => workItems.filter((item) => item.stage_id === stage.id),
    [workItems, stage.id],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const cachedEnvironments = peekProductEnvironments(workspaceId, productId);
      const envPromise =
        cachedEnvironments != null
          ? Promise.resolve(cachedEnvironments)
          : prefetchProductEnvironments(workspaceId, productId);
      const [wf, projectItems, envItems] = await Promise.all([
        product.getProductWorkflow(workspaceId, productId, activeWorkflowId),
        product.listProductProjects(workspaceId, productId),
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
  }, [activeWorkflowId, productId, workspaceId]);

  useEffect(() => {
    if (!open) {
      setWorkflow(null);
      setProjects([]);
      setEnvironments([]);
      setError(null);
      setBusy(false);
      setLoading(false);
      setSelectedIds(new Set());
      return;
    }
    setActiveWorkflowId(workflowId);
    setSelectedIds(new Set([initialSelectedId]));
    prefetchWorkspaceRunAssets(workspaceId);
  }, [open, workflowId, initialSelectedId, workspaceId]);

  useEffect(() => {
    if (!open) return;
    void load();
  }, [open, load]);

  useEffect(() => {
    if (!open) return;
    setSelectedIds((prev) => {
      const allowed = new Set(selectableItems.map((item) => item.id));
      const next = new Set([...prev].filter((id) => allowed.has(id)));
      if (next.size === 0 && selectableItems.length > 0) {
        next.add(selectableItems[0].id);
      }
      return next;
    });
  }, [open, selectableItems]);

  function toggleItem(itemId: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(itemId)) {
        if (next.size <= 1) return prev;
        next.delete(itemId);
      } else {
        next.add(itemId);
      }
      return next;
    });
  }

  function selectAll() {
    setSelectedIds(new Set(selectableItems.map((item) => item.id)));
  }

  function clearSelection() {
    if (selectableItems.length === 0) return;
    setSelectedIds(new Set([selectableItems[0].id]));
  }

  async function onSubmit(input: StartProductWorkflowRunCommand["input"]) {
    if (!workflow || selectedIds.size === 0) return;
    setBusy(true);
    setError(null);
    try {
      for (const itemId of selectedIds) {
        await product.executeWorkItem(workspaceId, productId, itemId, {
          stage_id: stage.id,
          workflow_id: activeWorkflowId,
          input,
        });
      }
      onClose();
      onExecuted();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "启动执行失败");
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;

  const workflowLabel =
    stageWorkflows.find((binding) => binding.workflow_id === activeWorkflowId)?.workflow_name ??
    workflow?.name;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="执行工作流"
      description={workflowLabel}
      size="full"
      priority
      panelClassName="sprint-wf-exec-modal"
    >
      <div className="sprint-wf-exec-layout">
        <aside className="sprint-wf-exec-sidebar" aria-label="选择工作项">
          <div className="sprint-wf-exec-sidebar-head">
            <h3 className="sprint-wf-exec-sidebar-title">选择工作项</h3>
          </div>
          <ul className="sprint-wf-exec-item-list" role="listbox" aria-multiselectable="true">
            {selectableItems.length === 0 ? (
              <li className="sprint-wf-exec-item-empty text-sm text-muted-foreground">当前阶段暂无可执行工作项</li>
            ) : (
              selectableItems.map((item) => {
                const checked = selectedIds.has(item.id);
                return (
                  <li key={item.id}>
                    <label className={`sprint-wf-exec-item${checked ? " is-selected" : ""}`}>
                      <input
                        type="checkbox"
                        className="sprint-wf-exec-item-check"
                        checked={checked}
                        onChange={() => toggleItem(item.id)}
                      />
                      <span className="sprint-wf-exec-item-title">{item.title}</span>
                    </label>
                  </li>
                );
              })
            )}
          </ul>
          <div className="sprint-wf-exec-sidebar-foot">
            <button type="button" className="btn-ghost text-xs" onClick={selectAll} disabled={selectableItems.length === 0}>
              全选
            </button>
            <button type="button" className="btn-ghost text-xs" onClick={clearSelection} disabled={selectableItems.length === 0}>
              取消全选
            </button>
          </div>
        </aside>

        <main className="sprint-wf-exec-main">
          {stageWorkflows.length > 1 ? (
            <div className="sprint-wf-exec-workflow-picker">
              <label className="stack-xs">
                <span className="text-sm font-medium">工作流</span>
                <select
                  className="input text-sm"
                  value={activeWorkflowId}
                  disabled={busy || loading}
                  onChange={(e) => setActiveWorkflowId(e.target.value)}
                >
                  {stageWorkflows.map((binding) => (
                    <option key={binding.workflow_id} value={binding.workflow_id}>
                      {binding.workflow_name ?? binding.workflow_id}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          ) : null}

          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

          {!loading && workflow && workflow.status !== "active" ? (
            <div className="empty-state text-sm">该工作流暂不可执行。</div>
          ) : loading || !workflow ? (
            <WorkflowRunFormSkeleton />
          ) : (
            <StartWorkflowRunModal
              open
              presentation="embedded"
              workspaceId={workspaceId}
              workflow={workflow}
              environments={environments}
              projects={projects}
              busy={busy || selectedIds.size === 0}
              loading={false}
              submitError={null}
              onClose={onClose}
              onSubmit={(input) => void onSubmit(input)}
            />
          )}
        </main>
      </div>
    </Modal>
  );
}
