"use client";

import { useCallback, useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ReleasePlanTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ReleasePlanFromTemplateModal } from "./ReleasePlanFromTemplateModal";

interface ReleasePlanTemplatesPanelProps {
  workspaceId: string;
  onPlanCreated?: (productId: string, planId: string) => void;
}

export function ReleasePlanTemplatesPanel({ workspaceId, onPlanCreated }: ReleasePlanTemplatesPanelProps) {
  const [templates, setTemplates] = useState<ReleasePlanTemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createTemplateId, setCreateTemplateId] = useState<string | undefined>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setTemplates(await product.listReleasePlanTemplates(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onDelete(templateId: string) {
    if (!window.confirm("确定删除该模板？")) return;
    setBusyId(templateId);
    try {
      await product.deleteReleasePlanTemplate(workspaceId, templateId);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="stack-sm border rounded-lg p-4 bg-surface">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium">工作区模板</h3>
          <p className="text-xs text-muted-foreground">复用已保存的发布项结构，快速创建新计划</p>
        </div>
        <button
          type="button"
          className="btn btn-primary text-xs"
          disabled={loading || templates.length === 0}
          onClick={() => {
            setCreateTemplateId(undefined);
            setCreateOpen(true);
          }}
        >
          从模板创建
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <p className="text-xs text-muted-foreground py-2">加载模板…</p>
      ) : templates.length === 0 ? (
        <p className="text-xs text-muted-foreground py-2">
          暂无模板。在产品页发布计划详情中点击「存为模板」即可保存。
        </p>
      ) : (
        <ul className="stack-xs text-sm mt-1">
          {templates.map((tpl) => (
            <li key={tpl.id} className="flex items-center justify-between gap-2 py-2 border-b last:border-0">
              <div className="stack-xs min-w-0">
                <span className="font-medium truncate">{tpl.name}</span>
                <span className="text-xs text-muted-foreground">
                  {tpl.job_count ?? 0} 个发布项
                  {tpl.created_at ? ` · ${new Date(tpl.created_at).toLocaleDateString()}` : ""}
                </span>
                {tpl.description ? <span className="text-xs text-muted-foreground truncate">{tpl.description}</span> : null}
              </div>
              <div className="flex gap-1 shrink-0">
                <button
                  type="button"
                  className="btn-secondary text-xs"
                  disabled={busyId === tpl.id}
                  onClick={() => {
                    setCreateTemplateId(tpl.id);
                    setCreateOpen(true);
                  }}
                >
                  使用
                </button>
                <button
                  type="button"
                  className="btn-secondary text-xs text-red-600"
                  disabled={busyId === tpl.id}
                  onClick={() => void onDelete(tpl.id)}
                >
                  删除
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

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
    </div>
  );
}
