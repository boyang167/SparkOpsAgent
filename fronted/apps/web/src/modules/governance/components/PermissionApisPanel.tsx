"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RegisteredAPI } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MetricCard } from "@/components/ui/MetricCard";
import { Modal } from "@/components/ui/Modal";
import { ApiEditModal } from "./apis/ApiEditModal";
import { ApiSyncModal, type SyncRow } from "./apis/ApiSyncModal";
import {
  collectApiGroups,
  computeApiStats,
  EMPTY_API_FILTERS,
  filterApis,
  HTTP_METHODS,
  methodBadgeClass,
  methodLabel,
  normalizeRegisteredAPI,
  normalizeApiSyncDiff,
  PAGE_SIZES,
  sortApis,
  type ApiSearchFilters,
  type ApiSortKey,
  type ApiSortOrder,
} from "./apis/api-utils";

interface PermissionApisPanelProps {
  workspaceId: string;
  canManage: boolean;
}

function formatError(err: unknown, fallback: string) {
  if (err instanceof BffError) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

export function PermissionApisPanel({ workspaceId, canManage }: PermissionApisPanelProps) {
  const [apis, setApis] = useState<RegisteredAPI[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [actionId, setActionId] = useState<string | null>(null);

  const [filters, setFilters] = useState<ApiSearchFilters>(EMPTY_API_FILTERS);
  const [appliedFilters, setAppliedFilters] = useState<ApiSearchFilters>(EMPTY_API_FILTERS);
  const [groupTab, setGroupTab] = useState<string>("all");
  const [sortKey, setSortKey] = useState<ApiSortKey>("api_group");
  const [sortOrder, setSortOrder] = useState<ApiSortOrder>("asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(PAGE_SIZES[0]);

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);

  const [syncOpen, setSyncOpen] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [newApis, setNewApis] = useState<SyncRow[]>([]);
  const [deleteApis, setDeleteApis] = useState<SyncRow[]>([]);
  const [ignoreApis, setIgnoreApis] = useState<SyncRow[]>([]);

  const [editTarget, setEditTarget] = useState<RegisteredAPI | null>(null);
  const [editGroup, setEditGroup] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editSaving, setEditSaving] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<RegisteredAPI | null>(null);

  const stats = useMemo(() => computeApiStats(apis), [apis]);
  const groups = useMemo(() => collectApiGroups(apis), [apis]);

  const filteredApis = useMemo(() => {
    let rows = filterApis(apis, appliedFilters);
    if (groupTab !== "all") {
      rows = rows.filter((api) => (api.api_group || "default") === groupTab);
    }
    return sortApis(rows, sortKey, sortOrder);
  }, [apis, appliedFilters, groupTab, sortKey, sortOrder]);

  const total = filteredApis.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pageRows = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredApis.slice(start, start + pageSize);
  }, [filteredApis, currentPage, pageSize]);

  const allPageSelected =
    pageRows.length > 0 && pageRows.every((row) => selectedIds.has(row.id));

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await governance.listWorkspaceApis(workspaceId);
      setApis(items.map((item) => normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>)));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3200);
  }

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setAppliedFilters(filters);
    setPage(1);
  }

  function onResetFilters() {
    setFilters(EMPTY_API_FILTERS);
    setAppliedFilters(EMPTY_API_FILTERS);
    setGroupTab("all");
    setPage(1);
  }

  function toggleSort(key: ApiSortKey) {
    if (sortKey === key) {
      setSortOrder((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortOrder("asc");
    }
  }

  function toggleSelectAllPage() {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (allPageSelected) {
        for (const row of pageRows) next.delete(row.id);
      } else {
        for (const row of pageRows) next.add(row.id);
      }
      return next;
    });
  }

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function openSync() {
    setSyncLoading(true);
    setError(null);
    try {
      const diff = normalizeApiSyncDiff(await governance.previewApiSync(workspaceId));
      setNewApis(diff.new);
      setDeleteApis(diff.delete);
      setIgnoreApis(diff.ignore);
      setSyncOpen(true);
    } catch (err) {
      setError(formatError(err, "同步预览失败"));
    } finally {
      setSyncLoading(false);
    }
  }

  async function confirmSync(selectedDeletes: SyncRow[] = deleteApis) {
    setSyncLoading(true);
    try {
      await governance.confirmApiSync(workspaceId, { new: newApis, delete: selectedDeletes });
      setSyncOpen(false);
      flashSuccess("API 同步完成");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "同步失败");
    } finally {
      setSyncLoading(false);
    }
  }

  function openEdit(row: RegisteredAPI) {
    setEditTarget(row);
    setEditGroup(row.api_group || "default");
    setEditDescription(row.description || "");
  }

  async function onSaveEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editTarget) return;
    setEditSaving(true);
    setError(null);
    try {
      await governance.updateApiMeta(workspaceId, editTarget.id, {
        api_group: editGroup.trim(),
        description: editDescription.trim(),
      });
      setEditTarget(null);
      flashSuccess("API 已更新");
      await load();
    } catch (err) {
      setError(formatError(err, "保存失败"));
    } finally {
      setEditSaving(false);
    }
  }

  async function confirmDeleteOne() {
    if (!deleteTarget) return;
    setActionId(deleteTarget.id);
    setError(null);
    try {
      await governance.deleteApi(workspaceId, deleteTarget.id);
      setSelectedIds((prev) => {
        const next = new Set(prev);
        next.delete(deleteTarget.id);
        return next;
      });
      setDeleteTarget(null);
      flashSuccess("API 已删除");
      await load();
    } catch (err) {
      setError(formatError(err, "删除失败"));
    } finally {
      setActionId(null);
    }
  }

  async function confirmBatchDelete() {
    const ids = Array.from(selectedIds);
    if (ids.length === 0) return;
    setActionId("batch");
    setError(null);
    try {
      await governance.deleteApisByIds(workspaceId, { ids });
      setSelectedIds(new Set());
      setBatchDeleteOpen(false);
      flashSuccess(`已删除 ${ids.length} 条 API`);
      await load();
    } catch (err) {
      setError(formatError(err, "批量删除失败"));
    } finally {
      setActionId(null);
    }
  }

  function sortIndicator(key: ApiSortKey) {
    if (sortKey !== key) return "";
    return sortOrder === "asc" ? " ↑" : " ↓";
  }

  return (
    <div className="stack-lg">
      <div className="menu-admin-toolbar-top">
        <p className="text-sm text-muted">
          管理全局 API 注册表，为角色授权提供接口清单。拥有 governance:admin 权限即可同步、编辑与删除。
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="btn-secondary" onClick={() => void load()} disabled={loading}>
            刷新
          </button>
          {canManage ? (
            <button type="button" className="btn" disabled={syncLoading} onClick={() => void openSync()}>
              {syncLoading ? "扫描中…" : "同步 API"}
            </button>
          ) : null}
        </div>
      </div>

      <div className="menu-admin-stats">
        <MetricCard
          label="API 总数"
          value={String(stats.total)}
          hint={`${stats.groups} 个分组`}
          tone="sky"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 7h16M4 12h10M4 17h14" strokeLinecap="round" />
            </svg>
          }
        />
        <MetricCard
          label="API 分组"
          value={String(stats.groups)}
          hint="按业务模块归类"
          tone="emerald"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="3" y="3" width="7" height="7" rx="1" />
              <rect x="14" y="3" width="7" height="7" rx="1" />
              <rect x="3" y="14" width="7" height="7" rx="1" />
              <rect x="14" y="14" width="7" height="7" rx="1" />
            </svg>
          }
        />
        <MetricCard
          label="GET 接口"
          value={String(stats.getCount)}
          hint="只读查询类"
          tone="violet"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
            </svg>
          }
        />
        <MetricCard
          label="最多方法"
          value={stats.topMethod.split(" ")[0] ?? "-"}
          hint={stats.topMethod.includes("(") ? stats.topMethod : "暂无数据"}
          tone="amber"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 3v18M8 8l4-4 4 4M8 16l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          }
        />
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <div className="card menu-admin-main">
        <form className="menu-admin-toolbar" onSubmit={onSearch}>
          <input
            className="input flex-1 min-w-[10rem]"
            placeholder="路径"
            value={filters.path}
            onChange={(e) => setFilters((prev) => ({ ...prev, path: e.target.value }))}
          />
          <input
            className="input flex-1 min-w-[10rem]"
            placeholder="分组"
            value={filters.api_group}
            onChange={(e) => setFilters((prev) => ({ ...prev, api_group: e.target.value }))}
          />
          <select
            className="input !w-auto min-w-[7rem]"
            value={filters.method}
            onChange={(e) => setFilters((prev) => ({ ...prev, method: e.target.value }))}
          >
            <option value="">全部方法</option>
            {HTTP_METHODS.map((method) => (
              <option key={method} value={method}>
                {method}
              </option>
            ))}
          </select>
          <input
            className="input flex-1 min-w-[10rem]"
            placeholder="描述"
            value={filters.description}
            onChange={(e) => setFilters((prev) => ({ ...prev, description: e.target.value }))}
          />
          <button type="submit" className="btn">
            查询
          </button>
          <button type="button" className="btn-secondary" onClick={onResetFilters}>
            重置
          </button>
        </form>

        {groups.length > 0 ? (
          <div className="flex flex-wrap gap-2 px-4 py-3 border-b border-[var(--border)]">
            <button
              type="button"
              className={`text-xs px-3 py-1 rounded-full border ${groupTab === "all" ? "bg-sky-100 border-sky-300 text-sky-800" : "border-[var(--border)] text-muted"}`}
              onClick={() => {
                setGroupTab("all");
                setPage(1);
              }}
            >
              全部分组
            </button>
            {groups.map((group) => (
              <button
                key={group}
                type="button"
                className={`text-xs px-3 py-1 rounded-full border ${groupTab === group ? "bg-sky-100 border-sky-300 text-sky-800" : "border-[var(--border)] text-muted"}`}
                onClick={() => {
                  setGroupTab(group);
                  setPage(1);
                }}
              >
                {group}
              </button>
            ))}
          </div>
        ) : null}

        {canManage && selectedIds.size > 0 ? (
          <div className="flex items-center gap-3 px-4 py-2 border-b border-[var(--border)] bg-slate-50">
            <span className="text-sm text-muted">已选 {selectedIds.size} 项</span>
            <button
              type="button"
              className="btn !bg-red-600 hover:!bg-red-700 !py-1 !px-3 text-xs"
              onClick={() => setBatchDeleteOpen(true)}
            >
              批量删除
            </button>
          </div>
        ) : null}

        {loading ? (
          <div className="menu-admin-skeleton">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="menu-admin-skeleton-row" style={{ width: `${92 - i * 4}%` }} />
            ))}
          </div>
        ) : total === 0 ? (
          <div className="menu-admin-empty">
            <p>{appliedFilters.path || appliedFilters.api_group || appliedFilters.method || appliedFilters.description || groupTab !== "all" ? "没有匹配的 API" : "暂无 API 记录"}</p>
            {canManage && !appliedFilters.path ? (
              <button type="button" className="btn mt-3" onClick={() => void openSync()}>
                同步后端路由
              </button>
            ) : null}
          </div>
        ) : (
          <>
            <div className="table-wrap">
              <table className="data-table text-sm">
                <thead>
                  <tr>
                    {canManage ? (
                      <th className="w-10">
                        <input type="checkbox" checked={allPageSelected} onChange={toggleSelectAllPage} />
                      </th>
                    ) : null}
                    <th>
                      <button type="button" className="font-medium hover:underline" onClick={() => toggleSort("api_group")}>
                        分组{sortIndicator("api_group")}
                      </button>
                    </th>
                    <th>
                      <button type="button" className="font-medium hover:underline" onClick={() => toggleSort("method")}>
                        方法{sortIndicator("method")}
                      </button>
                    </th>
                    <th>
                      <button type="button" className="font-medium hover:underline" onClick={() => toggleSort("path")}>
                        路径{sortIndicator("path")}
                      </button>
                    </th>
                    <th>
                      <button type="button" className="font-medium hover:underline" onClick={() => toggleSort("description")}>
                        描述{sortIndicator("description")}
                      </button>
                    </th>
                    {canManage ? <th className="w-28">操作</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {pageRows.map((row) => (
                    <tr key={row.id || `${row.method}:${row.path}`}>
                      {canManage ? (
                        <td>
                          <input
                            type="checkbox"
                            checked={selectedIds.has(row.id)}
                            onChange={() => toggleSelect(row.id)}
                          />
                        </td>
                      ) : null}
                      <td>{row.api_group || "default"}</td>
                      <td>
                        <span className={`badge ${methodBadgeClass(row.method)}`}>
                          {row.method} / {methodLabel(row.method)}
                        </span>
                      </td>
                      <td className="font-mono text-xs break-all">{row.path}</td>
                      <td>{row.description || "-"}</td>
                      {canManage ? (
                        <td>
                          <div className="flex items-center gap-2">
                            <button type="button" className="text-sky-700 hover:underline text-xs" onClick={() => openEdit(row)}>
                              编辑
                            </button>
                            <button
                              type="button"
                              className="text-red-600 hover:underline text-xs disabled:opacity-50"
                              disabled={actionId === row.id}
                              onClick={() => setDeleteTarget(row)}
                            >
                              删除
                            </button>
                          </div>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3 border-t border-[var(--border)]">
              <p className="text-sm text-muted">共 {total} 条</p>
              <div className="flex flex-wrap items-center gap-3">
                <label className="flex items-center gap-2 text-sm text-muted">
                  每页
                  <select
                    className="input !py-1 !px-2 !w-auto"
                    value={pageSize}
                    onChange={(e) => {
                      setPageSize(Number(e.target.value));
                      setPage(1);
                    }}
                  >
                    {PAGE_SIZES.map((size) => (
                      <option key={size} value={size}>
                        {size}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    className="btn-secondary !py-1 !px-2 text-xs"
                    disabled={currentPage <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                  >
                    上一页
                  </button>
                  <span className="text-sm text-muted">
                    {currentPage} / {totalPages}
                  </span>
                  <button
                    type="button"
                    className="btn-secondary !py-1 !px-2 text-xs"
                    disabled={currentPage >= totalPages}
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  >
                    下一页
                  </button>
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      <ApiSyncModal
        open={syncOpen}
        loading={syncLoading}
        newApis={newApis}
        deleteApis={deleteApis}
        ignoreApis={ignoreApis}
        onClose={() => setSyncOpen(false)}
        onConfirm={({ deleteApis: selectedDeletes }) => void confirmSync(selectedDeletes)}
        onChangeNewApi={(index, patch) =>
          setNewApis((prev) => prev.map((item, i) => (i === index ? { ...item, ...patch } : item)))
        }
        onApplyBatch={(patch) =>
          setNewApis((prev) =>
            prev.map((item) => ({
              ...item,
              ...(patch.api_group !== undefined ? { api_group: patch.api_group } : {}),
              ...(patch.description !== undefined ? { description: patch.description } : {}),
            })),
          )
        }
      />

      <ApiEditModal
        open={Boolean(editTarget)}
        api={editTarget}
        apiGroup={editGroup}
        description={editDescription}
        saving={editSaving}
        onClose={() => setEditTarget(null)}
        onChangeGroup={setEditGroup}
        onChangeDescription={setEditDescription}
        onSubmit={onSaveEdit}
      />

      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="确认删除 API"
        description={
          deleteTarget
            ? `将永久删除 ${deleteTarget.method} ${deleteTarget.path}，所有角色下该 API 的授权也会失效。`
            : undefined
        }
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setDeleteTarget(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn !bg-red-600 hover:!bg-red-700"
              disabled={Boolean(actionId)}
              onClick={() => void confirmDeleteOne()}
            >
              {actionId ? "删除中…" : "确定删除"}
            </button>
          </div>
        }
      >
        <p className="text-sm text-muted">若该接口仍存在于后端路由中，下次同步会再次出现在「新增」列表。</p>
      </Modal>

      <Modal
        open={batchDeleteOpen}
        onClose={() => setBatchDeleteOpen(false)}
        title="确认批量删除"
        description={`确定要删除选中的 ${selectedIds.size} 条 API 吗？`}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setBatchDeleteOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn !bg-red-600 hover:!bg-red-700"
              disabled={actionId === "batch"}
              onClick={() => void confirmBatchDelete()}
            >
              {actionId === "batch" ? "删除中…" : "确定删除"}
            </button>
          </div>
        }
      >
        <p className="text-sm text-muted">此操作不可撤销，请确认这些 API 不再需要授权管理。</p>
      </Modal>
    </div>
  );
}
