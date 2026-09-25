"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  ProductWorkflowVM,
  SprintProcessVM,
  SprintStage,
  WorkspaceSprintProcessTemplateVM,
} from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { SprintStageEditor } from "./SprintStageEditor";
import { normalizeStageForSave, SPRINT_PROCESS_TEMPLATES, withSortOrder } from "./sprint-process-templates";

interface SprintProcessPanelProps {
  workspaceId: string;
  productId: string;
}

export function SprintProcessPanel({ workspaceId, productId }: SprintProcessPanelProps) {
  const [processes, setProcesses] = useState<SprintProcessVM[]>([]);
  const [wsTemplates, setWsTemplates] = useState<WorkspaceSprintProcessTemplateVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [stages, setStages] = useState<SprintStage[]>([]);
  const [workflows, setWorkflows] = useState<ProductWorkflowVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [newTemplateOpen, setNewTemplateOpen] = useState(false);
  const [newTemplateName, setNewTemplateName] = useState("");

  const selected = useMemo(
    () => processes.find((item) => item.id === selectedId) ?? null,
    [processes, selectedId],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setSaved(false);
    try {
      const [procItems, wfItems, wsItems] = await Promise.all([
        product.listSprintProcesses(workspaceId, productId),
        product.listProductWorkflows(workspaceId, productId),
        product.listWorkspaceSprintProcessTemplates(workspaceId),
      ]);
      setProcesses(procItems);
      setWorkflows(wfItems);
      setWsTemplates(wsItems);
      setSelectedId((prev) => {
        if (prev && procItems.some((item) => item.id === prev)) return prev;
        return procItems.find((item) => item.is_default)?.id ?? procItems[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载迭代流程失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!selected) {
      setName("");
      setStages([]);
      return;
    }
    setName(selected.name);
    setStages(selected.stages ?? []);
    setSaved(false);
  }, [selected]);

  function applyBuiltinTemplate(templateId: string) {
    const template = SPRINT_PROCESS_TEMPLATES.find((item) => item.id === templateId);
    if (!template) return;
    setStages(withSortOrder(template.stages.map((stage) => ({ ...stage, id: crypto.randomUUID() }))));
    setSaved(false);
  }

  async function onSave() {
    if (!selected || !name.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const payload = {
        name: name.trim(),
        stages: withSortOrder(stages).map((stage, index) => {
          const normalized = normalizeStageForSave(stage, index);
          return {
            id: normalized.id,
            key: normalized.key,
            name: normalized.name,
            sort_order: normalized.sort_order,
            color: normalized.color,
            wip_limit: normalized.wip_limit,
            entry_policy: normalized.entry_policy,
            workflows: normalized.workflows ?? [],
          };
        }),
      };
      const savedProcess = selected.is_default
        ? await product.upsertSprintProcess(workspaceId, productId, payload)
        : await product.updateSprintProcessById(workspaceId, productId, selected.id, payload);
      setProcesses((prev) => prev.map((item) => (item.id === savedProcess.id ? savedProcess : item)));
      setStages(savedProcess.stages ?? []);
      setSaved(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function onCreateTemplate() {
    if (!newTemplateName.trim() || stages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      const created = await product.createSprintProcess(workspaceId, productId, {
        name: newTemplateName.trim(),
        stages: withSortOrder(stages).map((stage, index) => {
          const normalized = normalizeStageForSave(stage, index);
          return {
            id: normalized.id,
            key: normalized.key,
            name: normalized.name,
            sort_order: normalized.sort_order,
            color: normalized.color,
            wip_limit: normalized.wip_limit,
            workflows: normalized.workflows ?? [],
          };
        }),
      });
      setNewTemplateOpen(false);
      setNewTemplateName("");
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDeleteTemplate() {
    if (!selected || selected.is_default) return;
    setBusy(true);
    setError(null);
    try {
      await product.deleteSprintProcess(workspaceId, productId, selected.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveToWorkspace() {
    if (!name.trim() || stages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      await product.createWorkspaceSprintProcessTemplate(workspaceId, {
        name: name.trim(),
        stages: withSortOrder(stages).map((stage, index) => {
          const normalized = normalizeStageForSave(stage, index);
          return {
            id: normalized.id,
            key: normalized.key,
            name: normalized.name,
            sort_order: normalized.sort_order,
            color: normalized.color,
            wip_limit: normalized.wip_limit,
            workflows: normalized.workflows ?? [],
          };
        }),
      });
      setWsTemplates(await product.listWorkspaceSprintProcessTemplates(workspaceId));
      setSaved(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存到工作区失败");
    } finally {
      setBusy(false);
    }
  }

  async function onImportWorkspaceTemplate(templateId: string) {
    setBusy(true);
    setError(null);
    try {
      const created = await product.importWorkspaceSprintTemplate(workspaceId, productId, {
        workspace_template_id: templateId,
      });
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "导入工作区模板失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return <div className="empty-state">加载迭代流程…</div>;
  }

  return (
    <div className="product-master-main-inner stack-md max-w-3xl">
      <div className="stack-xs">
        <h2 className="section-title">迭代流程</h2>
        <p className="text-sm text-muted-foreground">
          管理产品级流程模板。保存后仅影响使用该模板<strong>新建</strong>的迭代。
        </p>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {saved ? <InlineAlert tone="success">已保存。</InlineAlert> : null}

      <div className="flex flex-wrap items-end gap-3">
        <label className="stack-xs min-w-[220px]">
          <span className="text-sm">流程模板</span>
          <select className="input" value={selectedId ?? ""} onChange={(e) => setSelectedId(e.target.value || null)}>
            {processes.length === 0 ? <option value="">无模板</option> : null}
            {processes.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}{item.is_default ? "（默认）" : ""} · v{item.version}
              </option>
            ))}
          </select>
        </label>
        <label className="stack-xs min-w-[200px]">
          <span className="text-sm">内置模板</span>
          <select className="input" defaultValue="" onChange={(e) => { if (e.target.value) applyBuiltinTemplate(e.target.value); e.target.value = ""; }}>
            <option value="">应用到当前编辑…</option>
            {SPRINT_PROCESS_TEMPLATES.map((template) => (
              <option key={template.id} value={template.id}>{template.label}</option>
            ))}
          </select>
        </label>
        {wsTemplates.length > 0 ? (
          <label className="stack-xs min-w-[200px]">
            <span className="text-sm">工作区模板</span>
            <select className="input" defaultValue="" onChange={(e) => { if (e.target.value) void onImportWorkspaceTemplate(e.target.value); e.target.value = ""; }}>
              <option value="">导入到产品…</option>
              {wsTemplates.map((template) => (
                <option key={template.id} value={template.id}>{template.name}</option>
              ))}
            </select>
          </label>
        ) : null}
      </div>

      {selected ? (
        <>
          <label className="stack-xs">
            <span className="text-sm">流程名称</span>
            <input className="input" value={name} onChange={(e) => { setName(e.target.value); setSaved(false); }} />
          </label>
          <SprintStageEditor stages={stages} workflows={workflows} onChange={(next) => { setStages(next); setSaved(false); }} />
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn btn-primary text-sm" disabled={busy || !name.trim()} onClick={() => void onSave()}>
              {busy ? "保存中…" : "保存流程"}
            </button>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setNewTemplateOpen(true)}>
              另存为新模板
            </button>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => void onSaveToWorkspace()}>
              保存到工作区库
            </button>
            {!selected.is_default ? (
              <button type="button" className="btn-ghost text-sm text-red-600" disabled={busy} onClick={() => void onDeleteTemplate()}>
                删除模板
              </button>
            ) : null}
          </div>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">暂无流程模板</p>
      )}

      <Modal
        open={newTemplateOpen}
        onClose={() => setNewTemplateOpen(false)}
        title="另存为新模板"
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" onClick={() => setNewTemplateOpen(false)}>取消</button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy || !newTemplateName.trim()} onClick={() => void onCreateTemplate()}>
              创建
            </button>
          </>
        }
      >
        <label className="stack-xs">
          <span className="text-sm">模板名称</span>
          <input className="input" value={newTemplateName} onChange={(e) => setNewTemplateName(e.target.value)} placeholder="例如 DevOps 六阶段" />
        </label>
      </Modal>
    </div>
  );
}
