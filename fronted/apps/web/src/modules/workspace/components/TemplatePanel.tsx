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
import { paginateItems } from "./template/template-pagination";
import {
  filterTemplates,
  formatDate,
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

const SORT_OPTIONS: { value: `${SortField}:${SortOrder}`; label: string }[] = [
  { value: "created_at:desc", label: "最新创建" },
  { value: "created_at:asc", label: "最早创建" },
  { value: "name:asc", label: "名称 A–Z" },
  { value: "name:desc", label: "名称 Z–A" },
  { value: "status:asc", label: "按状态" },
];

export function TemplatePanel({ workspaceId }: TemplatePanelProps) {
  const keywordRef = useRef<HTMLInputElement>(null);
  const [items, setItems] = useState<TemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>("card");
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
  const [pendingDelete, setPendingDelete] = useState<TemplateVM | null>(null);
  const [deleting, setDeleting] = useState(false);

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

  const hasFilters = Boolean(keyword.trim() || kindFilter || languageFilter || statusFilter);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(
    () => paginateItems(filtered, page, pageSize),
    [filtered, page, pageSize],
  );

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
  }

  function onSortChange(value: string) {
    const [field, order] = value.split(":") as [SortField, SortOrder];
    setSortField(field);
    setSortOrder(order);
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

  async function confirmDelete() {
    const item = pendingDelete;
    if (!item) return;
    setDeleting(true);
    setError(null);
    try {
      await product.deleteTemplate(workspaceId, item.id);
      if (selected?.id === item.id) setDetailOpen(false);
      setPendingDelete(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setDeleting(false);
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

  return (
    <div className="template-panel">
      <div className="template-toolbar">
        <div className="template-filters">
          <label className="template-search">
            <SearchIcon />
            <input
              ref={keywordRef}
              className="template-search-input"
              placeholder="搜索名称或内容"
              aria-label="搜索模板"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
          </label>
          <select
            className="input template-filter-select"
            aria-label="分类"
            value={kindFilter}
            onChange={(e) => setKindFilter(e.target.value)}
          >
            <option value="">分类</option>
            {TEMPLATE_KIND_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
          <select
            className="input template-filter-select"
            aria-label="语言"
            value={languageFilter}
            onChange={(e) => setLanguageFilter(e.target.value)}
          >
            <option value="">语言</option>
            {TEMPLATE_LANGUAGE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
          <select
            className="input template-filter-select"
            aria-label="状态"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="">状态</option>
            {TEMPLATE_STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
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
          创建模板
        </button>
      </div>

      <div className="template-result-bar">
        <p className="template-result-count">
          {loading ? "加载中…" : `${filtered.length} 个模板`}
          {hasFilters && !loading ? (
            <button type="button" className="template-clear-filters" onClick={resetFilters}>
              清除筛选
            </button>
          ) : null}
        </p>
        <div className="template-view-actions">
          <div className="template-view-switch" role="group" aria-label="视图">
            <button
              type="button"
              className={viewMode === "card" ? "is-active" : ""}
              aria-pressed={viewMode === "card"}
              aria-label="卡片视图"
              title="卡片视图"
              onClick={() => setViewMode("card")}
            >
              <GridIcon />
            </button>
            <button
              type="button"
              className={viewMode === "table" ? "is-active" : ""}
              aria-pressed={viewMode === "table"}
              aria-label="表格视图"
              title="表格视图"
              onClick={() => setViewMode("table")}
            >
              <ListIcon />
            </button>
          </div>
          <button
            type="button"
            className="template-icon-btn"
            onClick={() => void load()}
            disabled={loading}
            aria-label="刷新"
            title="刷新"
          >
            <RefreshIcon />
          </button>
        </div>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="template-card-grid" role="status" aria-label="加载模板">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="template-card template-card-skeleton">
              <div className="template-skeleton-line short" />
              <div className="template-skeleton-line" />
              <div className="template-skeleton-block" />
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="template-empty">
          {hasFilters ? (
            <>
              <p className="template-empty-title">没有符合条件的模板</p>
              <p className="template-empty-desc">试试换个关键词，或清除筛选条件。</p>
              <button type="button" className="btn-secondary" onClick={resetFilters}>清除筛选</button>
            </>
          ) : (
            <>
              <p className="template-empty-title">还没有模板</p>
              <p className="template-empty-desc">模板用于生成项目的部署清单与流水线。</p>
              <button type="button" className="btn-primary" onClick={openCreate}>创建第一个模板</button>
            </>
          )}
        </div>
      ) : viewMode === "card" ? (
        <>
          <div className="template-card-grid">
            {paged.map((item) => (
              <TemplateCard
                key={item.id}
                template={item}
                onView={openDetail}
                onEdit={openEdit}
                onCopy={(t) => void onCopy(t)}
                onDelete={setPendingDelete}
              />
            ))}
          </div>
          <TemplatePagination
            page={page}
            pageSize={pageSize}
            totalPages={totalPages}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
          />
        </>
      ) : (
        <div className="template-table-shell">
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
                  <th aria-label="操作" />
                </tr>
              </thead>
              <tbody>
                {paged.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <button type="button" className="template-table-name" onClick={() => openDetail(item)}>
                        {item.name}
                      </button>
                    </td>
                    <td className="template-table-muted">{labelOf(TEMPLATE_KIND_OPTIONS, item.kind)}</td>
                    <td className="template-table-muted">{labelOf(TEMPLATE_LANGUAGE_OPTIONS, item.language)}</td>
                    <td><span className={statusBadgeClass(item.status)}>{labelOf(TEMPLATE_STATUS_OPTIONS, item.status)}</span></td>
                    <td className="template-table-muted">{templateVisibilityLabel(item)}</td>
                    <td><TemplateCreator template={item} compact /></td>
                    <td className="template-table-muted">{formatDate(item.created_at)}</td>
                    <td>
                      <div className="template-row-actions">
                        {templateIsEditable(item) ? (
                          <>
                            <button type="button" onClick={() => openEdit(item)}>编辑</button>
                            <button type="button" onClick={() => void onCopy(item)}>复制</button>
                            <button type="button" className="is-danger" onClick={() => setPendingDelete(item)}>删除</button>
                          </>
                        ) : (
                          <button type="button" onClick={() => void onCopy(item)}>复制</button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <TemplatePagination
            page={page}
            pageSize={pageSize}
            totalPages={totalPages}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
          />
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

      <Modal
        open={Boolean(pendingDelete)}
        onClose={() => (deleting ? undefined : setPendingDelete(null))}
        title="删除模板"
        description={pendingDelete ? `确定删除「${pendingDelete.name}」？已引用该模板的项目将回退到系统默认内容。` : undefined}
        size="sm"
        footer={
          <>
            <button type="button" className="btn-secondary" disabled={deleting} onClick={() => setPendingDelete(null)}>
              取消
            </button>
            <button type="button" className="btn-danger-solid" disabled={deleting} onClick={() => void confirmDelete()}>
              {deleting ? "删除中…" : "删除"}
            </button>
          </>
        }
      >
        <p className="template-delete-hint">删除后无法恢复。</p>
      </Modal>
    </div>
  );
}

function TemplatePagination({
  page,
  pageSize,
  totalPages,
  onPageChange,
  onPageSizeChange,
}: {
  page: number;
  pageSize: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  return (
    <div className="template-pagination">
      <select
        className="input template-page-size"
        aria-label="每页条数"
        value={pageSize}
        onChange={(event) => onPageSizeChange(Number(event.target.value))}
      >
        {PAGE_SIZES.map((size) => (
          <option key={size} value={size}>{size} 条/页</option>
        ))}
      </select>
      <button type="button" className="btn-ghost" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
        上一页
      </button>
      <span className="template-page-indicator">{page} / {totalPages}</span>
      <button
        type="button"
        className="btn-ghost"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
      >
        下一页
      </button>
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
    <svg className="template-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" strokeLinecap="round" />
    </svg>
  );
}

function GridIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <rect x="4" y="4" width="6.5" height="6.5" rx="1.5" />
      <rect x="13.5" y="4" width="6.5" height="6.5" rx="1.5" />
      <rect x="4" y="13.5" width="6.5" height="6.5" rx="1.5" />
      <rect x="13.5" y="13.5" width="6.5" height="6.5" rx="1.5" />
    </svg>
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="M8 6h12M8 12h12M8 18h12" strokeLinecap="round" />
      <circle cx="4" cy="6" r="1" fill="currentColor" />
      <circle cx="4" cy="12" r="1" fill="currentColor" />
      <circle cx="4" cy="18" r="1" fill="currentColor" />
    </svg>
  );
}

function RefreshIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="M20 11a8 8 0 1 0-2.3 5.7" strokeLinecap="round" />
      <path d="M20 4v7h-7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
