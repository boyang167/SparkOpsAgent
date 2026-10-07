"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SprintStage, WorkspaceSprintProcessTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { SprintStageEditor } from "./SprintStageEditor";
import { normalizeStageForSave, SPRINT_PROCESS_TEMPLATES, withSortOrder } from "./sprint-process-templates";

interface WorkspaceSprintProcessPanelProps {
  workspaceId: string;
}

const FALLBACK_STAGE_COLOR = "#86868b";

function stageFlow(stages: SprintStage[] | undefined) {
  return (stages ?? []).map((stage) => stage.name || "未命名").join(" → ");
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
  const [newStages, setNewStages] = useState<SprintStage[]>([]);
  const [saved, setSaved] = useState(true);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

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
      setError(err instanceof BffError ? err.message : "加载迭代流程模板失败");
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

  useEffect(() => {
    if (!menuOpen) return;
    function onDocClick(e: MouseEvent) {
      if (!menuRef.current?.contains(e.target as Node)) setMenuOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [menuOpen]);

  const visibleTemplates = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return templates;
    return templates.filter(
      (item) => item.name.toLowerCase().includes(q) || stageFlow(item.stages).toLowerCase().includes(q),
    );
  }, [templates, searchQuery]);

  function freshStages(source: SprintStage[]) {
    return withSortOrder(source.map((stage) => ({ ...stage, id: crypto.randomUUID() })));
  }

  function applyPresetToEditor(presetId: string) {
    const preset = SPRINT_PROCESS_TEMPLATES.find((item) => item.id === presetId);
    if (!preset) return;
    setStages(freshStages(preset.stages));
    setSaved(false);
  }

  function applyPresetToCreate(presetId: string) {
    const preset = SPRINT_PROCESS_TEMPLATES.find((item) => item.id === presetId);
    if (!preset) return;
    setNewName((current) => current.trim() || preset.label);
    setNewStages(freshStages(preset.stages));
  }

  function toPayload(source: SprintStage[]) {
    return withSortOrder(source).map((stage, index) => {
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
    setNewStages(freshStages(SPRINT_PROCESS_TEMPLATES[0].stages));
    setCreateOpen(true);
  }

  function openCreateFromCurrent() {
    setMenuOpen(false);
    setNewName(name.trim() ? `${name.trim()} 副本` : "");
    setNewStages(freshStages(stages));
    setCreateOpen(true);
  }

  async function onCreateTemplate() {
    if (!newName.trim() || newStages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      const created = await product.createWorkspaceSprintProcessTemplate(workspaceId, {
        name: newName.trim(),
        stages: toPayload(newStages),
      });
      setCreateOpen(false);
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
        stages: toPayload(stages),
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

  function discardChanges() {
    if (!selected) return;
    setName(selected.name);
    setStages(selected.stages ?? []);
    setSaved(true);
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

  const canSave = !busy && !saved && Boolean(name.trim()) && stages.length >= 2;

  return (
    <div className="sp-tpl">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="sp-tpl-layout">
        <aside className="sp-tpl-sidebar" aria-label="流程模板">
          <div className="sp-tpl-sidebar-head">
            <span className="sp-tpl-sidebar-title">
              流程模板
              {!loading ? <span className="sp-tpl-sidebar-count">{templates.length}</span> : null}
            </span>
            <button type="button" className="sp-tpl-icon-btn" onClick={openCreateModal} aria-label="新建流程模板" title="新建流程模板">
              <PlusIcon />
            </button>
          </div>

          {templates.length > 4 ? (
            <label className="sp-tpl-search">
              <SearchIcon />
              <input
                type="search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="搜索模板"
                aria-label="搜索模板"
              />
            </label>
          ) : null}

          <div className="sp-tpl-list" role="listbox" aria-label="流程模板列表">
            {loading ? (
              Array.from({ length: 3 }).map((_, i) => <div key={i} className="sp-tpl-item is-skeleton" />)
            ) : visibleTemplates.length === 0 ? (
              <p className="sp-tpl-list-empty">{templates.length === 0 ? "还没有流程模板" : "没有匹配的模板"}</p>
            ) : (
              visibleTemplates.map((item) => {
                const active = item.id === selectedId;
                const itemStages = item.stages ?? [];
                return (
                  <button
                    key={item.id}
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`sp-tpl-item${active ? " is-active" : ""}`}
                    onClick={() => setSelectedId(item.id)}
                  >
                    <span className="sp-tpl-item-row">
                      <span className="sp-tpl-item-name">{item.name}</span>
                      <span className="sp-tpl-item-meta">{itemStages.length} 列</span>
                    </span>
                    <span className="sp-tpl-item-strip" aria-hidden="true">
                      {itemStages.map((stage) => (
                        <span key={stage.id} style={{ backgroundColor: stage.color || FALLBACK_STAGE_COLOR }} />
                      ))}
                    </span>
                    <span className="sp-tpl-item-flow">{stageFlow(itemStages)}</span>
                  </button>
                );
              })
            )}
          </div>
        </aside>

        <section className="sp-tpl-main">
          {loading ? (
            <div className="sp-tpl-main-skeleton" role="status" aria-label="加载中">
              <div className="template-skeleton-line short" />
              <div className="sp-tpl-skeleton-block" />
            </div>
          ) : !selected ? (
            <div className="sp-tpl-empty">
              <p className="sp-tpl-empty-title">{templates.length === 0 ? "还没有迭代流程模板" : "选择一个模板开始编辑"}</p>
              <p className="sp-tpl-empty-desc">流程模板定义迭代看板的列，新建迭代时可直接选用。</p>
              {templates.length === 0 ? (
                <button type="button" className="btn-primary" onClick={openCreateModal}>新建流程模板</button>
              ) : null}
            </div>
          ) : (
            <>
              <header className="sp-tpl-head">
                <div className="sp-tpl-head-copy">
                  <input
                    className="sp-tpl-name-input"
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value);
                      setSaved(false);
                    }}
                    placeholder="模板名称"
                    aria-label="模板名称"
                  />
                  <p className="sp-tpl-head-sub">
                    {stages.length} 列 · 修改不影响已创建的迭代
                  </p>
                </div>
                <div className="sp-tpl-head-actions">
                  {!saved ? (
                    <>
                      <span className="sp-tpl-dirty">未保存</span>
                      <button type="button" className="btn-ghost" disabled={busy} onClick={discardChanges}>
                        放弃
                      </button>
                    </>
                  ) : null}
                  <button type="button" className="btn-primary" disabled={!canSave} onClick={() => void onSaveTemplate()}>
                    {busy ? "保存中…" : "保存"}
                  </button>
                  <div className="sp-tpl-menu-wrap" ref={menuRef}>
                    <button
                      type="button"
                      className="sp-tpl-icon-btn"
                      aria-label="更多操作"
                      aria-expanded={menuOpen}
                      onClick={() => setMenuOpen((v) => !v)}
                    >
                      <MoreIcon />
                    </button>
                    {menuOpen ? (
                      <div className="sp-tpl-menu" role="menu">
                        <span className="sp-tpl-menu-label">用预设替换列</span>
                        {SPRINT_PROCESS_TEMPLATES.map((preset) => (
                          <button
                            key={preset.id}
                            type="button"
                            role="menuitem"
                            onClick={() => {
                              setMenuOpen(false);
                              applyPresetToEditor(preset.id);
                            }}
                          >
                            {preset.label}
                          </button>
                        ))}
                        <span className="sp-tpl-menu-divider" />
                        <button type="button" role="menuitem" onClick={openCreateFromCurrent}>另存为新模板</button>
                        <button
                          type="button"
                          role="menuitem"
                          className="is-danger"
                          onClick={() => {
                            setMenuOpen(false);
                            setDeleteOpen(true);
                          }}
                        >
                          删除模板
                        </button>
                      </div>
                    ) : null}
                  </div>
                </div>
              </header>

              <SprintStageEditor
                stages={stages}
                workflows={[]}
                onChange={(next) => {
                  setStages(next);
                  setSaved(false);
                }}
                variant="board"
              />
            </>
          )}
        </section>
      </div>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建流程模板"
        description="从预设开始，再按团队习惯调整列。"
        size="lg"
        footer={
          <>
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={busy || !newName.trim() || newStages.length < 2}
              onClick={() => void onCreateTemplate()}
            >
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-md">
          <label className="stack-xs">
            <span className="label">模板名称</span>
            <input
              className="input"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="例如 标准敏捷"
            />
          </label>
          <div className="stack-xs">
            <span className="label">从预设开始</span>
            <div className="sp-tpl-presets">
              {SPRINT_PROCESS_TEMPLATES.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className="sp-tpl-preset"
                  onClick={() => applyPresetToCreate(preset.id)}
                >
                  <span className="sp-tpl-item-strip" aria-hidden="true">
                    {preset.stages.map((stage, i) => (
                      <span key={i} style={{ backgroundColor: stage.color || FALLBACK_STAGE_COLOR }} />
                    ))}
                  </span>
                  <span className="sp-tpl-preset-name">{preset.label}</span>
                </button>
              ))}
            </div>
          </div>
          <SprintStageEditor stages={newStages} workflows={[]} onChange={setNewStages} variant="board" />
        </div>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={() => (busy ? undefined : setDeleteOpen(false))}
        title="删除流程模板"
        description={`确定删除「${selected?.name ?? ""}」？已创建的迭代不会受影响。`}
        size="sm"
        footer={
          <>
            <button type="button" className="btn-secondary" disabled={busy} onClick={() => setDeleteOpen(false)}>
              取消
            </button>
            <button type="button" className="btn-danger-solid" disabled={busy} onClick={() => void onDeleteTemplate()}>
              {busy ? "删除中…" : "删除"}
            </button>
          </>
        }
      >
        <p className="sp-tpl-delete-hint">删除后，新建迭代时将无法再选择此模板。</p>
      </Modal>
    </div>
  );
}

function PlusIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M12 5v14M5 12h14" strokeLinecap="round" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" strokeLinecap="round" />
    </svg>
  );
}

function MoreIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
      <circle cx="5" cy="12" r="1.6" />
      <circle cx="12" cy="12" r="1.6" />
      <circle cx="19" cy="12" r="1.6" />
    </svg>
  );
}
