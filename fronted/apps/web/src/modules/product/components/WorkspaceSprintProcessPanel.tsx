"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SprintStage, WorkspaceSprintProcessTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductMasterDetail } from "./ProductMasterDetail";
import { SprintStageEditor } from "./SprintStageEditor";
import { normalizeStageForSave, SPRINT_PROCESS_TEMPLATES, withSortOrder } from "./sprint-process-templates";

interface WorkspaceSprintProcessPanelProps {
  workspaceId: string;
}

export function WorkspaceSprintProcessPanel({ workspaceId }: WorkspaceSprintProcessPanelProps) {
  const [templates, setTemplates] = useState<WorkspaceSprintProcessTemplateVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [stages, setStages] = useState<SprintStage[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [saved, setSaved] = useState(true);

  const selected = useMemo(
    () => templates.find((item) => item.id === selectedId) ?? null,
    [templates, selectedId],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await product.listWorkspaceSprintProcessTemplates(workspaceId);
      setTemplates(items);
      setSelectedId((prev) => {
        if (prev && items.some((item) => item.id === prev)) return prev;
        return items[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作区流程模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

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
    setSaved(true);
  }, [selected]);

  const listItems = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return templates
      .filter((item) => !q || item.name.toLowerCase().includes(q))
      .map((item) => ({
        id: item.id,
        title: item.name,
        meta: `${item.stages?.length ?? 0} 阶段`,
        subtitle: item.description?.trim() || undefined,
      }));
  }, [templates, searchQuery]);

  function applyBuiltinTemplate(templateId: string, target: "edit" | "create") {
    const template = SPRINT_PROCESS_TEMPLATES.find((item) => item.id === templateId);
    if (!template) return;
    const nextStages = withSortOrder(template.stages.map((stage) => ({ ...stage, id: crypto.randomUUID() })));
    if (target === "create") {
      setNewName(template.label);
      setStages(nextStages);
      return;
    }
    setStages(nextStages);
    setSaved(false);
  }

  function stagePayload() {
    return withSortOrder(stages).map((stage, index) => {
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
    });
  }

  function openCreateModal() {
    setNewName("");
    setStages(withSortOrder(SPRINT_PROCESS_TEMPLATES[0].stages.map((s) => ({ ...s, id: crypto.randomUUID() }))));
    setCreateOpen(true);
  }

  async function onCreateTemplate() {
    if (!newName.trim() || stages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      const created = await product.createWorkspaceSprintProcessTemplate(workspaceId, {
        name: newName.trim(),
        stages: stagePayload(),
      });
      setCreateOpen(false);
      setNewName("");
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveTemplate() {
    if (!selected || !name.trim() || stages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await product.updateWorkspaceSprintProcessTemplate(workspaceId, selected.id, {
        name: name.trim(),
        stages: stagePayload(),
      });
      setTemplates((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      setName(updated.name);
      setStages(updated.stages ?? []);
      setSaved(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDeleteTemplate() {
    if (!selected) return;
    setBusy(true);
    setError(null);
    try {
      await product.deleteWorkspaceSprintProcessTemplate(workspaceId, selected.id);
      setDeleteOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除模板失败");
    } finally {
      setBusy(false);
    }
  }

  function openCreateFromCurrent() {
    setNewName(name.trim() ? `${name.trim()} 副本` : "");
    setStages(withSortOrder(stages.map((s) => ({ ...s, id: crypto.randomUUID() }))));
    setCreateOpen(true);
  }

  if (loading) {
    return (
      <div className="ws-sprint-template-shell">
        <div className="empty-state" role="status">
          加载迭代流程模板…
        </div>
      </div>
    );
  }

  return (
    <div className="ws-sprint-template-shell">
      {error ? (
        <div className="ws-sprint-template-alert">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="流程模板"
        searchPlaceholder="搜索模板"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={openCreateModal}
        addAriaLabel="新建流程模板"
        listEmpty={
          templates.length === 0 ? (
            <div className="ws-sprint-template-list-empty">
              <p>暂无模板</p>
              <button type="button" className="btn btn-primary text-sm" onClick={openCreateModal}>
                新建模板
              </button>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground px-3 py-4">无匹配模板</p>
          )
        }
        showMain={Boolean(selected)}
        emptyMain={
          <div className="empty-state">
            <p>选择左侧模板开始编辑</p>
          </div>
        }
      >
        {selected ? (
          <div className="ws-sprint-template-editor">
            <header className="ws-sprint-template-head">
              <div className="ws-sprint-template-head-main stack-xs">
                <label className="ws-sprint-template-name-field">
                  <span className="sr-only">模板名称</span>
                  <input
                    className="ws-sprint-template-name-input"
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value);
                      setSaved(false);
                    }}
                    placeholder="模板名称"
                  />
                </label>
                {selected.description ? (
                  <p className="ws-sprint-template-desc">{selected.description}</p>
                ) : null}
              </div>
              <div className="ws-sprint-template-head-actions">
                {!saved ? <span className="ws-sprint-template-badge">未保存</span> : null}
                <label className="ws-sprint-template-preset">
                  <span className="sr-only">从内置模板填充</span>
                  <select
                    className="input text-sm"
                    defaultValue=""
                    onChange={(e) => {
                      if (e.target.value) applyBuiltinTemplate(e.target.value, "edit");
                      e.target.value = "";
                    }}
                  >
                    <option value="">从预设填充…</option>
                    {SPRINT_PROCESS_TEMPLATES.map((template) => (
                      <option key={template.id} value={template.id}>
                        {template.label}
                      </option>
                    ))}
                  </select>
                </label>
                <button
                  type="button"
                  className="btn btn-primary text-sm"
                  disabled={busy || saved || !name.trim() || stages.length < 2}
                  onClick={() => void onSaveTemplate()}
                >
                  {busy ? "保存中…" : "保存"}
                </button>
                <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={openCreateFromCurrent}>
                  另存为
                </button>
                <button type="button" className="btn-danger-ghost text-sm" disabled={busy} onClick={() => setDeleteOpen(true)}>
                  删除
                </button>
              </div>
            </header>

            <section className="ws-sprint-template-stages card">
              <SprintStageEditor
                stages={stages}
                workflows={[]}
                onChange={(next) => {
                  setStages(next);
                  setSaved(false);
                }}
                variant="board"
              />
            </section>

            <p className="ws-sprint-template-footnote text-sm text-muted-foreground">
              修改模板不影响已创建迭代；新建迭代时可选择此处模板。
            </p>
          </div>
        ) : null}
      </ProductMasterDetail>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建流程模板"
        description="基于预设快速创建，保存后可在新建迭代时选用。"
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || !newName.trim() || stages.length < 2}
              onClick={() => void onCreateTemplate()}
            >
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-md">
          <label className="stack-xs">
            <span className="text-sm">模板名称</span>
            <input
              className="input"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="例如 标准敏捷（4 列）"
            />
          </label>
          <label className="stack-xs">
            <span className="text-sm">内置预设</span>
            <select
              className="input"
              defaultValue=""
              onChange={(e) => {
                if (e.target.value) applyBuiltinTemplate(e.target.value, "create");
                e.target.value = "";
              }}
            >
              <option value="">选择预设填充阶段…</option>
              {SPRINT_PROCESS_TEMPLATES.map((template) => (
                <option key={template.id} value={template.id}>
                  {template.label}
                </option>
              ))}
            </select>
          </label>
          <SprintStageEditor stages={stages} workflows={[]} onChange={setStages} variant="board" />
        </div>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="删除流程模板"
        description={`确定删除「${selected?.name ?? ""}」？已创建的迭代不会受影响。`}
        footer={
          <>
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setDeleteOpen(false)}>
              取消
            </button>
            <button type="button" className="btn-danger-solid text-sm" disabled={busy} onClick={() => void onDeleteTemplate()}>
              {busy ? "删除中…" : "确认删除"}
            </button>
          </>
        }
      >
        <p className="text-sm text-muted-foreground">删除后，新建迭代时将无法选择此模板。</p>
      </Modal>
    </div>
  );
}
