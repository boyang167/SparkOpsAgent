"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateTemplateCommand, TemplateVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { TemplateCard } from "./template/TemplateCard";
import { TemplateCreator } from "./template/TemplateCreator";
import { TemplateDetail } from "./template/TemplateDetail";
import { TemplateForm } from "./template/TemplateForm";
import {
  filterTemplates,
  formatDateTime,
  labelOf,
  sortTemplates,
  statusBadgeClass,
  templateIsEditable,
  templateVisibilityLabel,
  TEMPLATE_KIND_OPTIONS,
  TEMPLATE_LANGUAGE_OPTIONS,
  TEMPLATE_STATUS_OPTIONS,
  type SortField,
  type SortOrder,
  type TableDensity,
} from "./template/template-utils";

interface TemplatePanelProps {
  workspaceId: string;
}

type ViewMode = "card" | "table";

const EMPTY_FORM: CreateTemplateCommand = {
  name: "",
  kind: "Deployment",
  language: "go",
  detail: "",
  rules: "",
  status: "active",
  is_public: false,
};

const PAGE_SIZES = [10, 20, 50, 100];

export function TemplatePanel({ workspaceId }: TemplatePanelProps) {
  const keywordRef = useRef<HTMLInputElement>(null);
  const [items, setItems] = useState<TemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>("card");
  const [density, setDensity] = useState<TableDensity>("comfortable");
  const [keyword, setKeyword] = useState("");
  const [kindFilter, setKindFilter] = useState("");
  const [languageFilter, setLanguageFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [sortField, setSortField] = useState<SortField>("created_at");
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [formOpen, setFormOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [selected, setSelected] = useState<TemplateVM | null>(null);
  const [form, setForm] = useState<CreateTemplateCommand>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listTemplates(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载模板失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    function onKeydown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && (e.key === "k" || e.key === "K")) {
        e.preventDefault();
        keywordRef.current?.focus();
      }
    }
    window.addEventListener("keydown", onKeydown);
    return () => window.removeEventListener("keydown", onKeydown);
  }, []);

  const filtered = useMemo(() => {
    const list = filterTemplates(items, {
      keyword,
      kind: kindFilter,
      language: languageFilter,
      status: statusFilter,
    });
    return sortTemplates(list, sortField, sortOrder);
  }, [items, keyword, kindFilter, languageFilter, statusFilter, sortField, sortOrder]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, page, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [keyword, kindFilter, languageFilter, statusFilter, sortField, sortOrder, pageSize]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  function resetFilters() {
    setKeyword("");
    setKindFilter("");
    setLanguageFilter("");
    setStatusFilter("");
    setSortField("created_at");
    setSortOrder("desc");
  }

  function openCreate() {
    setEditingId(null);
    setForm(EMPTY_FORM);
    setFormOpen(true);
  }

  function openEdit(item: TemplateVM) {
    if (!templateIsEditable(item)) return;
    setEditingId(item.id);
    setForm({
      name: item.name,
      kind: item.kind ?? "Deployment",
      language: item.language ?? "go",
      detail: item.detail ?? "",
      rules: item.rules ?? "",
      status: item.status ?? "active",
      is_public: item.is_public ?? false,
    });
    setFormOpen(true);
  }

  function openDetail(item: TemplateVM) {
    setSelected(item);
    setDetailOpen(true);
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      if (editingId) {
        await product.updateTemplate(workspaceId, editingId, form);
      } else {
        await product.createTemplate(workspaceId, form);
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function onDelete(item: TemplateVM) {
    if (!window.confirm(`确定删除模板「${item.name}」？`)) return;
    setError(null);
    try {
      await product.deleteTemplate(workspaceId, item.id);
      if (selected?.id === item.id) setDetailOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    }
  }

  async function onCopy(item: TemplateVM) {
    setError(null);
    try {
      await product.createTemplate(workspaceId, {
        name: `${item.name}_副本`,
        kind: item.kind ?? "Deployment",
        language: item.language ?? "go",
        detail: item.detail ?? "",
        rules: item.rules ?? "",
        status: "draft",
        is_public: false,
      });
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "复制失败");
    }
  }

  async function copyContent(text: string) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      setError("复制到剪贴板失败");
    }
  }

  function downloadContent(item: TemplateVM) {
    const blob = new Blob([item.detail ?? ""], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${item.name}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }

  const tableRows = viewMode === "table" ? paged : filtered;

  return (
    <div className="template-panel">
      <div className="template-toolbar">
        <span className="template-total-badge">总数：{filtered.length}</span>
        <button type="button" className="btn" onClick={openCreate}>
          <PlusIcon />
          创建模板
        </button>
      </div>

      <div className="template-search-bar">
        <div className="template-search-grid">
          <label className="template-search-field">
            <span>关键词</span>
            <div className="template-search-input-wrap">
              <input
                ref={keywordRef}
                className="input"
                placeholder="请输入模板名称或内容 (Ctrl+K)"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
              <button type="button" className="template-search-btn" aria-label="搜索">⌕</button>
            </div>
          </label>
          <label className="template-search-field">
            <span>分类</span>
            <select className="input" value={kindFilter} onChange={(e) => setKindFilter(e.target.value)}>
              <option value="">请选择分类</option>
              {TEMPLATE_KIND_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </label>
          <label className="template-search-field">
            <span>语言</span>
            <select className="input" value={languageFilter} onChange={(e) => setLanguageFilter(e.target.value)}>
              <option value="">请选择语言</option>
              {TEMPLATE_LANGUAGE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </label>
          <label className="template-search-field">
            <span>状态</span>
            <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="">请选择状态</option>
              {TEMPLATE_STATUS_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </label>
          <label className="template-search-field">
            <span>排序</span>
            <div className="template-sort-row">
              <select className="input" value={sortField} onChange={(e) => setSortField(e.target.value as SortField)}>
                <option value="created_at">创建时间</option>
                <option value="name">名称</option>
                <option value="status">状态</option>
              </select>
              <select className="input" value={sortOrder} onChange={(e) => setSortOrder(e.target.value as SortOrder)}>
                <option value="desc">降序</option>
                <option value="asc">升序</option>
              </select>
            </div>
          </label>
          <div className="template-search-field template-search-reset">
            <button type="button" className="btn-secondary" onClick={resetFilters}>重置</button>
          </div>
        </div>
      </div>

      <div className="template-view-controls">
        <div className="template-view-switch">
          <button type="button" className={viewMode === "card" ? "is-active" : ""} onClick={() => setViewMode("card")}>卡片视图</button>
          <button type="button" className={viewMode === "table" ? "is-active" : ""} onClick={() => setViewMode("table")}>表格视图</button>
        </div>
        <div className="template-view-actions">
          {viewMode === "table" ? (
            <div className="template-density-switch">
              <button type="button" className={density === "compact" ? "is-active" : ""} onClick={() => setDensity("compact")}>紧凑</button>
              <button type="button" className={density === "comfortable" ? "is-active" : ""} onClick={() => setDensity("comfortable")}>适中</button>
            </div>
          ) : null}
          <button type="button" className="btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
            ↻ 刷新
          </button>
        </div>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="template-skeleton">
          <div className="template-skeleton-line" />
          <div className="template-skeleton-line" />
          <div className="template-skeleton-line short" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="template-empty">
          <p>暂无模板</p>
          <button type="button" className="btn" onClick={openCreate}>创建第一个模板</button>
        </div>
      ) : viewMode === "card" ? (
        <div className="template-card-grid">
          {filtered.map((item) => (
            <TemplateCard
              key={item.id}
              template={item}
              onView={openDetail}
              onEdit={openEdit}
              onCopy={(t) => void onCopy(t)}
              onDelete={(t) => void onDelete(t)}
            />
          ))}
        </div>
      ) : (
        <div className={`template-table-shell density-${density}`}>
          <div className="table-wrap">
            <table className="data-table template-table text-sm">
              <thead>
                <tr>
                  <th>模板名称</th>
                  <th>分类</th>
                  <th>语言</th>
                  <th>状态</th>
                  <th>可见性</th>
                  <th>创建者</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {tableRows.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <div className="template-table-name">
                        <DocumentIcon />
                        <span>{item.name}</span>
                      </div>
                    </td>
                    <td><span className="badge badge-primary">{labelOf(TEMPLATE_KIND_OPTIONS, item.kind)}</span></td>
                    <td><span className="badge badge-neutral">{labelOf(TEMPLATE_LANGUAGE_OPTIONS, item.language)}</span></td>
                    <td><span className={statusBadgeClass(item.status)}>{labelOf(TEMPLATE_STATUS_OPTIONS, item.status)}</span></td>
                    <td><span className={`badge ${item.is_public || item.shared ? "badge-success" : "badge-neutral"}`}>{templateVisibilityLabel(item)}</span></td>
                    <td><TemplateCreator template={item} compact /></td>
                    <td>{formatDateTime(item.created_at)}</td>
                    <td>
                      <div className="template-row-actions">
                        <button type="button" onClick={() => openDetail(item)}>查看</button>
                        {templateIsEditable(item) ? (
                          <>
                            <button type="button" onClick={() => openEdit(item)}>编辑</button>
                            <button type="button" className="is-danger" onClick={() => void onDelete(item)}>删除</button>
                          </>
                        ) : null}
                        <button type="button" onClick={() => void onCopy(item)}>复制</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="template-pagination">
            <span className="text-sm text-muted-foreground">共 {filtered.length} 条</span>
            <select className="input template-page-size" value={pageSize} onChange={(e) => setPageSize(Number(e.target.value))}>
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>{size} 条/页</option>
              ))}
            </select>
            <button type="button" className="btn-secondary text-sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>上一页</button>
            <span className="text-sm">{page} / {totalPages}</span>
            <button type="button" className="btn-secondary text-sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>下一页</button>
          </div>
        </div>
      )}

      <Modal open={formOpen} onClose={() => setFormOpen(false)} title={editingId ? "编辑模板" : "创建模板"} size="lg">
        <TemplateForm
          form={form}
          isEdit={Boolean(editingId)}
          submitting={submitting}
          onChange={setForm}
          onSubmit={onSubmit}
          onCancel={() => setFormOpen(false)}
        />
      </Modal>

      <Modal open={detailOpen} onClose={() => setDetailOpen(false)} title="模板详情" size="xl">
        {selected ? (
          <TemplateDetail
            template={selected}
            onCopyContent={(text) => void copyContent(text)}
            onDownload={downloadContent}
          />
        ) : null}
      </Modal>
    </div>
  );
}

function PlusIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 5v14M5 12h14" strokeLinecap="round" />
    </svg>
  );
}

function DocumentIcon() {
  return (
    <svg className="template-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" strokeLinecap="round" />
    </svg>
  );
}
