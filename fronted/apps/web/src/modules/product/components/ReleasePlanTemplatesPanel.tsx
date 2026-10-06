"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ReleasePlanTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanFromTemplateModal } from "./ReleasePlanFromTemplateModal";

interface ReleasePlanTemplatesPanelProps {
  workspaceId: string;
  onPlanCreated?: (productId: string, planId: string) => void;
  onTemplatesLoaded?: (templates: ReleasePlanTemplateVM[]) => void;
  createOpen?: boolean;
  onCreateOpenChange?: (open: boolean) => void;
  initialTemplateId?: string;
}

export function ReleasePlanTemplatesPanel({
  workspaceId,
  onPlanCreated,
  onTemplatesLoaded,
  createOpen: controlledCreateOpen,
  onCreateOpenChange,
  initialTemplateId,
}: ReleasePlanTemplatesPanelProps) {
  const [templates, setTemplates] = useState<ReleasePlanTemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [internalCreateOpen, setInternalCreateOpen] = useState(false);
  const [createTemplateId, setCreateTemplateId] = useState<string | undefined>();
  const [deleteTarget, setDeleteTarget] = useState<ReleasePlanTemplateVM | null>(null);

  const createOpen = controlledCreateOpen ?? internalCreateOpen;
  const setCreateOpen = onCreateOpenChange ?? setInternalCreateOpen;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await product.listReleasePlanTemplates(workspaceId);
      setTemplates(items);
      onTemplatesLoaded?.(items);
      if (items.length > 0) setExpanded(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, onTemplatesLoaded]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (controlledCreateOpen) {
      setCreateTemplateId(initialTemplateId);
    }
  }, [controlledCreateOpen, initialTemplateId]);

  function openCreate(templateId?: string) {
    setCreateTemplateId(templateId);
    setCreateOpen(true);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setBusyId(deleteTarget.id);
    try {
      await product.deleteReleasePlanTemplate(workspaceId, deleteTarget.id);
      setDeleteTarget(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setBusyId(null);
    }
  }

  if (loading) {
    return (
      <p className="text-xs text-muted-foreground" role="status">
        加载模板…
      </p>
    );
  }

  if (templates.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        暂无工作区模板。可在产品发布计划详情中「存为模板」后在此快速复用。
      </p>
    );
  }

  return (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="btn-ghost text-xs px-0 text-muted-foreground hover:text-foreground"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
        >
          {expanded ? "收起模板" : `工作区模板（${templates.length}）`}
        </button>
        {!expanded ? (
          <div className="flex flex-wrap gap-1.5">
            {templates.slice(0, 3).map((tpl) => (
              <button
                key={tpl.id}
                type="button"
                className="badge badge-neutral cursor-pointer hover:opacity-80 transition-opacity"
                onClick={() => openCreate(tpl.id)}
              >
                {tpl.name}
              </button>
            ))}
            {templates.length > 3 ? <span className="text-xs text-muted-foreground">+{templates.length - 3}</span> : null}
          </div>
        ) : null}
      </div>
      {expanded ? (
        <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 mt-2">
          {templates.map((tpl) => (
            <li
              key={tpl.id}
              className="list-item cursor-pointer"
              onClick={() => openCreate(tpl.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  openCreate(tpl.id);
                }
              }}
              role="button"
              tabIndex={0}
            >
              <div className="stack-xs min-w-0 flex-1">
                <span className="font-medium truncate text-sm">{tpl.name}</span>
                <span className="text-xs text-muted-foreground">
                  {tpl.job_count ?? 0} 个发布项
                  {tpl.created_at ? ` · ${new Date(tpl.created_at).toLocaleDateString()}` : ""}
                </span>
              </div>
              <button
                type="button"
                className="btn-ghost text-xs text-muted-foreground shrink-0"
                aria-label={`删除模板 ${tpl.name}`}
                disabled={busyId === tpl.id}
                onClick={(e) => {
                  e.stopPropagation();
                  setDeleteTarget(tpl);
                }}
              >
                删除
              </button>
            </li>
          ))}
        </ul>
      ) : null}

      <ReleasePlanFromTemplateModal
        open={createOpen}
        workspaceId={workspaceId}
        templates={templates}
        initialTemplateId={createTemplateId}
        onClose={() => {
          setCreateOpen(false);
          setCreateTemplateId(undefined);
        }}
        onCreated={(pid, planId) => {
          void load();
          onPlanCreated?.(pid, planId);
        }}
      />

      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="删除模板"
        description={deleteTarget ? `确定删除「${deleteTarget.name}」？此操作不可恢复。` : undefined}
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setDeleteTarget(null)}>
              取消
            </button>
            <button type="button" className="btn-danger-solid text-sm" disabled={Boolean(busyId)} onClick={() => void confirmDelete()}>
              {busyId ? "删除中…" : "确认删除"}
            </button>
          </>
        }
      >
        <p className="text-sm text-muted-foreground">删除后不会影响已创建的发布计划。</p>
      </Modal>
    </>
  );
}
