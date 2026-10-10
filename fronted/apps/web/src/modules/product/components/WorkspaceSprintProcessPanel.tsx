"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SprintStage, WorkspaceSprintProcessTemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { SprintStageEditor } from "./SprintStageEditor";
import { normalizeStageForSave, SPRINT_PROCESS_TEMPLATES, withSortOrder } from "./sprint-process-templates";
import { paginateItems } from "@/modules/workspace/components/template/template-pagination";

interface WorkspaceSprintProcessPanelProps {
  workspaceId: string;
}

const FALLBACK_STAGE_COLOR = "#86868b";
const PAGE_SIZES = [10, 20, 50, 100];

const SORT_OPTIONS = [
  { value: "created_at:desc", label: "最新创建" },
  { value: "created_at:asc", label: "最早创建" },
  { value: "name:asc", label: "名称 A–Z" },
  { value: "name:desc", label: "名称 Z–A" },
  { value: "columns:desc", label: "列数多→少" },
  { value: "columns:asc", label: "列数少→多" },
] as const;

function stageFlow(stages: SprintStage[] | undefined) {
  return (stages ?? []).map((stage) => stage.name || "未命名").join(" → ");
}

function compareTemplates(
  a: WorkspaceSprintProcessTemplateVM,
  b: WorkspaceSprintProcessTemplateVM,
  field: string,
  order: "asc" | "desc",
) {
  const dir = order === "asc" ? 1 : -1;
  if (field === "name") return a.name.localeCompare(b.name) * dir;
  if (field === "columns") return ((a.stages?.length ?? 0) - (b.stages?.length ?? 0)) * dir;
  const at = a.created_at ?? "";
  const bt = b.created_at ?? "";
  return at.localeCompare(bt) * dir;
}

export function WorkspaceSprintProcessPanel({ workspaceId }: WorkspaceSprintProcessPanelProps) {
  const [templates, setTemplates] = useState<WorkspaceSprintProcessTemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [sortField, setSortField] = useState<string>("created_at");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formName, setFormName] = useState("");
  const [formStages, setFormStages] = useState<SprintStage[]>([]);

  const [pendingDelete, setPendingDelete] = useState<WorkspaceSprintProcessTemplateVM | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setTemplates(await product.listWorkspaceSprintProcessTemplates(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载迭代流程模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    const list = templates.filter((item) => {
      if (q && !(item.name.toLowerCase().includes(q) || stageFlow(item.stages).toLowerCase().includes(q))) {
        return false;
      }
      return true;
    });
    return [...list].sort((a, b) => compareTemplates(a, b, sortField, sortOrder));
  }, [templates, searchQuery, sortField, sortOrder]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => paginateItems(filtered, page, pageSize), [filtered, page, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [searchQuery, sortField, sortOrder, pageSize]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  function onSortChange(value: string) {
    const [field, order] = value.split(":") as [string, "asc" | "desc"];
    setSortField(field);
    setSortOrder(order);
  }

  function freshStages(source: SprintStage[]) {
    return withSortOrder(source.map((stage) => ({ ...stage, id: crypto.randomUUID() })));
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

  function applyPreset(presetId: string) {
    const preset = SPRINT_PROCESS_TEMPLATES.find((item) => item.id === presetId);
    if (!preset) return;
    setFormStages(freshStages(preset.stages));
    if (!editingId) setFormName((current) => current.trim() || preset.label);
  }

  function openCreate() {
    setEditingId(null);
    setFormName("");
    setFormStages(freshStages(SPRINT_PROCESS_TEMPLATES[0].stages));
    setFormOpen(true);
  }

  function openEdit(item: WorkspaceSprintProcessTemplateVM) {
    setEditingId(item.id);
    setFormName(item.name);
    setFormStages((item.stages ?? []).map((stage) => ({ ...stage })));
    setFormOpen(true);
  }

  function openCopy(item: WorkspaceSprintProcessTemplateVM) {
    setEditingId(null);
    setFormName(item.name.trim() ? `${item.name.trim()} 副本` : "");
    setFormStages(freshStages(item.stages ?? []));
    setFormOpen(true);
  }

  async function onSubmit() {
    if (!formName.trim() || formStages.length < 2) return;
    setBusy(true);
    setError(null);
    try {
      if (editingId) {
        await product.updateWorkspaceSprintProcessTemplate(workspaceId, editingId, {
          name: formName.trim(),
          stages: toPayload(formStages),
        });
      } else {
        await product.createWorkspaceSprintProcessTemplate(workspaceId, {
          name: formName.trim(),
          stages: toPayload(formStages),
        });
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存模板失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDeleteTemplate() {
    const item = pendingDelete;
    if (!item) return;
    setBusy(true);
    setError(null);
    try {
      await product.deleteWorkspaceSprintProcessTemplate(workspaceId, item.id);
      setPendingDelete(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除模板失败");
    } finally {
      setBusy(false);
    }
  }

  const hasFilters = Boolean(searchQuery.trim());

  function resetFilters() {
    setSearchQuery("");
  }

  return (
    <div className="template-panel">
      <div className="template-toolbar">
        <div className="template-filters">
          <label className="template-search">
            <SearchIcon />
            <input
              className="template-search-input"
              placeholder="搜索名称或流程"
              aria-label="搜索流程模板"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </label>
          <select
            className="input template-filter-select"
            aria-label="排序"
            value={`${sortField}:${sortOrder}`}
            onChange={(e) => onSortChange(e.target.value)}
          >
            {SORT_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
        <button type="button" className="btn-primary" onClick={openCreate}>
          <PlusIcon />
          新建流程模板
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="template-card-grid" role="status" aria-label="加载模板">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="template-card template-card-skeleton">
              <div className="template-skeleton-line short" />
              <div className="template-skeleton-line" />
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="template-empty">
          {hasFilters ? (
            <>
              <p className="template-empty-title">没有符合条件的模板</p>
              <p className="template-empty-desc">试试换个关键词或清除筛选。</p>
              <button type="button" className="btn-secondary" onClick={resetFilters}>清除筛选</button>
            </>
          ) : (
            <>
              <p className="template-empty-title">还没有流程模板</p>
              <p className="template-empty-desc">流程模板定义迭代看板的列，新建迭代时可直接选用。</p>
              <button type="button" className="btn-primary" onClick={openCreate}>创建第一个模板</button>
            </>
          )}
        </div>
      ) : (
        <>
          <div className="template-card-grid">
            {paged.map((item) => (
              <SprintTemplateCard
                key={item.id}
                template={item}
                onEdit={openEdit}
                onCopy={openCopy}
                onDelete={setPendingDelete}
              />
            ))}
          </div>
          <div className="template-pagination">
            <select
              className="input template-page-size"
              aria-label="每页条数"
              value={pageSize}
              onChange={(e) => setPageSize(Number(e.target.value))}
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>{size} 条/页</option>
              ))}
            </select>
            <button type="button" className="btn-ghost" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              上一页
            </button>
            <span className="template-page-indicator">{page} / {totalPages}</span>
            <button type="button" className="btn-ghost" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              下一页
            </button>
          </div>
        </>
      )}

      <Modal
        open={formOpen}
        onClose={() => (busy ? undefined : setFormOpen(false))}
        title={editingId ? "编辑流程模板" : "新建流程模板"}
        size="lg"
        footer={
          <>
            <button type="button" className="btn-secondary" disabled={busy} onClick={() => setFormOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={busy || !formName.trim() || formStages.length < 2}
              onClick={() => void onSubmit()}
            >
              {busy ? "保存中…" : editingId ? "保存" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-md">
          <label className="stack-xs">
            <span className="label">模板名称</span>
            <input
              className="input"
              value={formName}
              onChange={(e) => setFormName(e.target.value)}
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
                  onClick={() => applyPreset(preset.id)}
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
          <SprintStageEditor stages={formStages} workflows={[]} onChange={setFormStages} variant="board" />
        </div>
      </Modal>

      <Modal
        open={Boolean(pendingDelete)}
        onClose={() => (busy ? undefined : setPendingDelete(null))}
        title="删除流程模板"
        description={pendingDelete ? `确定删除「${pendingDelete.name}」？已创建的迭代不会受影响。` : undefined}
        size="sm"
        footer={
          <>
            <button type="button" className="btn-secondary" disabled={busy} onClick={() => setPendingDelete(null)}>
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

interface SprintTemplateCardProps {
  template: WorkspaceSprintProcessTemplateVM;
  onEdit: (template: WorkspaceSprintProcessTemplateVM) => void;
  onCopy: (template: WorkspaceSprintProcessTemplateVM) => void;
  onDelete: (template: WorkspaceSprintProcessTemplateVM) => void;
}

function SprintTemplateCard({ template, onEdit, onCopy, onDelete }: SprintTemplateCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const stages = template.stages ?? [];

  useEffect(() => {
    if (!menuOpen) return;
    function onDocClick(e: MouseEvent) {
      if (!menuRef.current?.contains(e.target as Node)) setMenuOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [menuOpen]);

  return (
    <article
      className={`template-card${menuOpen ? " is-menu-open" : ""}`}
      onClick={() => onEdit(template)}
      onKeyDown={(e) => {
        if (e.key === "Enter") onEdit(template);
      }}
      role="button"
      tabIndex={0}
    >
      <header className="template-card-header">
        <div className="template-card-heading">
          <span className="template-card-kind">{stages.length} 列</span>
          <h3 className="template-card-title" title={template.name}>{template.name}</h3>
        </div>
        <div className="template-card-actions" ref={menuRef} onClick={(e) => e.stopPropagation()}>
          <button
            type="button"
            className="template-card-menu-btn"
            aria-label="更多操作"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((v) => !v)}
          >
            <MoreIcon />
          </button>
          {menuOpen ? (
            <div className="template-card-menu" role="menu">
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onEdit(template); }}>编辑</button>
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); onCopy(template); }}>另存为新模板</button>
              <button type="button" role="menuitem" className="is-danger" onClick={() => { setMenuOpen(false); onDelete(template); }}>删除</button>
            </div>
          ) : null}
        </div>
      </header>

      <span className="sp-tpl-item-strip" aria-hidden="true">
        {stages.map((stage) => (
          <span key={stage.id} style={{ backgroundColor: stage.color || FALLBACK_STAGE_COLOR }} />
        ))}
      </span>
      <p className="sp-tpl-item-flow">{stageFlow(stages)}</p>
    </article>
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
