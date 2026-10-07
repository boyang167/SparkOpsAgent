"use client";

import { useEffect, useMemo, useState } from "react";
import type { RegisteredAPI } from "@sparkops/contracts/governance";
import { Modal } from "@/components/ui/Modal";
import { inferApiMeta, methodBadgeClass } from "./api-utils";

export type SyncRow = RegisteredAPI & { api_group?: string; description?: string };

type SyncTab = "new" | "delete" | "ignore";

interface ApiSyncModalProps {
  open: boolean;
  loading: boolean;
  newApis: SyncRow[];
  deleteApis: SyncRow[];
  ignoreApis: SyncRow[];
  onClose: () => void;
  onConfirm: (selection: { deleteApis: SyncRow[] }) => void;
  onChangeNewApi: (index: number, patch: Partial<SyncRow>) => void;
  onApplyBatch?: (patch: Partial<SyncRow>) => void;
}

function syncRowKey(row: SyncRow) {
  return row.id || `${row.method}:${row.path}`;
}

function rowGroup(row: SyncRow) {
  const group = row.api_group?.trim();
  if (group) return group;
  return inferApiMeta(row.path, row.method).api_group;
}

function groupRows(rows: SyncRow[]) {
  const map = new Map<string, SyncRow[]>();
  for (const row of rows) {
    const group = rowGroup(row);
    const list = map.get(group) ?? [];
    list.push(row);
    map.set(group, list);
  }
  return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b, "zh-CN"));
}

function filterRows(rows: SyncRow[], query: string) {
  const q = query.trim().toLowerCase();
  if (!q) return rows;
  return rows.filter(
    (row) =>
      row.path.toLowerCase().includes(q) ||
      row.method.toLowerCase().includes(q) ||
      (row.api_group ?? "").toLowerCase().includes(q) ||
      (row.description ?? "").toLowerCase().includes(q),
  );
}

function ApiRouteLine({ row }: { row: SyncRow }) {
  return (
    <div className="api-sync-route">
      <span className={`badge shrink-0 ${methodBadgeClass(row.method)}`}>{row.method}</span>
      <code className="api-sync-path">{row.path}</code>
    </div>
  );
}

export function ApiSyncModal({
  open,
  loading,
  newApis,
  deleteApis,
  ignoreApis,
  onClose,
  onConfirm,
  onChangeNewApi,
  onApplyBatch,
}: ApiSyncModalProps) {
  const [tab, setTab] = useState<SyncTab>("new");
  const [query, setQuery] = useState("");
  const [batchGroup, setBatchGroup] = useState("");
  const [batchDescription, setBatchDescription] = useState("");
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [selectedDeleteKeys, setSelectedDeleteKeys] = useState<Set<string>>(new Set());
  const deleteSnapshot = useMemo(() => deleteApis.map(syncRowKey).join("\n"), [deleteApis]);

  useEffect(() => {
    if (!open) return;
    setTab(newApis.length > 0 ? "new" : deleteApis.length > 0 ? "delete" : "ignore");
    setQuery("");
    setBatchGroup("");
    setBatchDescription("");
    setSelectedDeleteKeys(new Set(deleteApis.map(syncRowKey)));
    setCollapsedGroups({});
  }, [open, deleteSnapshot, newApis.length, deleteApis]);

  const filteredNew = useMemo(() => filterRows(newApis, query), [newApis, query]);
  const filteredDelete = useMemo(() => filterRows(deleteApis, query), [deleteApis, query]);
  const filteredIgnore = useMemo(() => filterRows(ignoreApis, query), [ignoreApis, query]);

  const groupedNew = useMemo(() => groupRows(filteredNew), [filteredNew]);
  const groupedDelete = useMemo(() => groupRows(filteredDelete), [filteredDelete]);
  const groupedIgnore = useMemo(() => groupRows(filteredIgnore), [filteredIgnore]);

  const selectedDeleteCount = useMemo(
    () => deleteApis.filter((row) => selectedDeleteKeys.has(syncRowKey(row))).length,
    [deleteApis, selectedDeleteKeys],
  );

  const visibleDeleteKeys = useMemo(() => filteredDelete.map(syncRowKey), [filteredDelete]);
  const allVisibleDeleteSelected =
    visibleDeleteKeys.length > 0 && visibleDeleteKeys.every((key) => selectedDeleteKeys.has(key));

  function toggleGroupCollapse(group: string) {
    setCollapsedGroups((prev) => ({ ...prev, [group]: !prev[group] }));
  }

  function isGroupCollapsed(group: string, count: number) {
    if (group in collapsedGroups) return collapsedGroups[group]!;
    return count > 8;
  }

  function applyBatch() {
    if (!onApplyBatch) return;
    onApplyBatch({
      ...(batchGroup.trim() ? { api_group: batchGroup.trim() } : {}),
      ...(batchDescription.trim() ? { description: batchDescription.trim() } : {}),
    });
  }

  function toggleDeleteKey(key: string) {
    setSelectedDeleteKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleVisibleDeletes() {
    setSelectedDeleteKeys((prev) => {
      const next = new Set(prev);
      if (allVisibleDeleteSelected) {
        for (const key of visibleDeleteKeys) next.delete(key);
      } else {
        for (const key of visibleDeleteKeys) next.add(key);
      }
      return next;
    });
  }

  function toggleDeleteGroup(rows: SyncRow[]) {
    const keys = rows.map(syncRowKey);
    const allSelected = keys.every((key) => selectedDeleteKeys.has(key));
    setSelectedDeleteKeys((prev) => {
      const next = new Set(prev);
      for (const key of keys) {
        if (allSelected) next.delete(key);
        else next.add(key);
      }
      return next;
    });
  }

  function handleConfirm() {
    const toDelete = deleteApis.filter((row) => selectedDeleteKeys.has(syncRowKey(row)));
    onConfirm({ deleteApis: toDelete });
  }

  const tabItems: Array<{ id: SyncTab; label: string; count: number; tone: string }> = [
    { id: "new", label: "新增", count: newApis.length, tone: "emerald" },
    { id: "delete", label: "删除", count: deleteApis.length, tone: "red" },
    { id: "ignore", label: "忽略", count: ignoreApis.length, tone: "amber" },
  ];

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="xl"
      panelClassName="api-sync-modal-panel"
      title="同步 API"
      description="扫描后端路由并与注册表对比差异。确认前请检查分组、描述及待删除项。"
      footer={
        <div className="api-sync-footer">
          <p className="api-sync-footer-summary">
            将<strong>新增 {newApis.length}</strong> 条
            {deleteApis.length > 0 ? (
              <>
                ，<strong>删除 {selectedDeleteCount}</strong> 条
              </>
            ) : null}
            {ignoreApis.length > 0 ? <>，忽略 {ignoreApis.length} 条不变</> : null}
          </p>
          <div className="flex gap-2 shrink-0">
            <button type="button" className="btn-secondary" disabled={loading} onClick={onClose}>
              取消
            </button>
            <button type="button" className="btn" disabled={loading} onClick={handleConfirm}>
              {loading ? "同步中…" : "确认同步"}
            </button>
          </div>
        </div>
      }
    >
      <div className="api-sync-modal">
        <div className="api-sync-summary">
          {tabItems.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`api-sync-summary-pill api-sync-summary-pill-${item.tone}${tab === item.id ? " api-sync-summary-pill-active" : ""}`}
              onClick={() => setTab(item.id)}
            >
              <span>{item.label}</span>
              <span className="api-sync-summary-count">{item.count}</span>
            </button>
          ))}
        </div>

        <div className="api-sync-toolbar">
          <div className="api-sync-search-wrap">
            <svg className="api-sync-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
            </svg>
            <input
              className="input api-sync-search"
              placeholder="搜索路径、分组或描述…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          {tab === "delete" && filteredDelete.length > 0 ? (
            <button type="button" className="btn-secondary text-sm shrink-0" onClick={toggleVisibleDeletes}>
              {allVisibleDeleteSelected ? "取消全选" : "全选当前结果"}
            </button>
          ) : null}
        </div>

        {tab === "new" ? (
          <div className="api-sync-panel">
            {newApis.length === 0 ? (
              <div className="api-sync-empty">无新增 API，注册表已与后端路由一致</div>
            ) : filteredNew.length === 0 ? (
              <div className="api-sync-empty">没有匹配「{query}」的新增 API</div>
            ) : (
              <>
                <p className="api-sync-hint">已根据路径自动填充分组与描述，可按分组展开后逐条微调。</p>
                {onApplyBatch ? (
                  <div className="api-sync-batch">
                    <p className="api-sync-batch-title">批量覆盖</p>
                    <div className="api-sync-batch-grid">
                      <input
                        className="input"
                        placeholder="默认分组"
                        value={batchGroup}
                        onChange={(e) => setBatchGroup(e.target.value)}
                      />
                      <input
                        className="input"
                        placeholder="默认描述"
                        value={batchDescription}
                        onChange={(e) => setBatchDescription(e.target.value)}
                      />
                      <button
                        type="button"
                        className="btn-secondary text-sm"
                        disabled={!batchGroup.trim() && !batchDescription.trim()}
                        onClick={applyBatch}
                      >
                        应用到全部新增
                      </button>
                    </div>
                  </div>
                ) : null}
                <div className="api-sync-groups">
                  {groupedNew.map(([group, rows]) => {
                    const collapsed = isGroupCollapsed(`new:${group}`, rows.length);
                    return (
                      <section key={group} className="api-sync-group">
                        <button type="button" className="api-sync-group-head" onClick={() => toggleGroupCollapse(`new:${group}`)}>
                          <span className="api-sync-group-chevron" data-collapsed={collapsed} aria-hidden>
                            ▾
                          </span>
                          <span className="api-sync-group-name">{group}</span>
                          <span className="api-sync-group-count">{rows.length}</span>
                        </button>
                        {!collapsed ? (
                          <div className="api-sync-group-body">
                            {rows.map((row) => {
                              const index = newApis.findIndex((item) => syncRowKey(item) === syncRowKey(row));
                              return (
                                <article key={syncRowKey(row)} className="api-sync-new-card">
                                  <ApiRouteLine row={row} />
                                  <div className="api-sync-meta-grid">
                                    <label className="api-sync-field">
                                      <span className="api-sync-field-label">分组</span>
                                      <input
                                        className="input"
                                        value={row.api_group ?? ""}
                                        onChange={(e) => onChangeNewApi(index, { api_group: e.target.value })}
                                      />
                                    </label>
                                    <label className="api-sync-field api-sync-field-wide">
                                      <span className="api-sync-field-label">描述</span>
                                      <input
                                        className="input"
                                        value={row.description ?? ""}
                                        onChange={(e) => onChangeNewApi(index, { description: e.target.value })}
                                      />
                                    </label>
                                  </div>
                                </article>
                              );
                            })}
                          </div>
                        ) : null}
                      </section>
                    );
                  })}
                </div>
              </>
            )}
          </div>
        ) : null}

        {tab === "delete" ? (
          <div className="api-sync-panel">
            {deleteApis.length === 0 ? (
              <div className="api-sync-empty">无待删除 API</div>
            ) : filteredDelete.length === 0 ? (
              <div className="api-sync-empty">没有匹配「{query}」的待删除 API</div>
            ) : (
              <>
                <p className="api-sync-hint api-sync-hint-warn">
                  以下 API 已不在后端路由中。取消勾选可保留在注册表（不参与本次删除）。
                </p>
                <div className="api-sync-groups">
                  {groupedDelete.map(([group, rows]) => {
                    const collapsed = isGroupCollapsed(`delete:${group}`, rows.length);
                    const groupKeys = rows.map(syncRowKey);
                    const groupSelected = groupKeys.filter((key) => selectedDeleteKeys.has(key)).length;
                    return (
                      <section key={group} className="api-sync-group">
                        <div className="api-sync-group-head api-sync-group-head-delete">
                          <label className="api-sync-check">
                            <input
                              type="checkbox"
                              checked={groupKeys.length > 0 && groupSelected === groupKeys.length}
                              ref={(el) => {
                                if (el) el.indeterminate = groupSelected > 0 && groupSelected < groupKeys.length;
                              }}
                              onChange={() => toggleDeleteGroup(rows)}
                            />
                          </label>
                          <button type="button" className="api-sync-group-toggle" onClick={() => toggleGroupCollapse(`delete:${group}`)}>
                            <span className="api-sync-group-chevron" data-collapsed={collapsed} aria-hidden>
                              ▾
                            </span>
                            <span className="api-sync-group-name">{group}</span>
                            <span className="api-sync-group-count">
                              {groupSelected}/{rows.length}
                            </span>
                          </button>
                        </div>
                        {!collapsed ? (
                          <div className="api-sync-group-body">
                            {rows.map((row) => {
                              const key = syncRowKey(row);
                              return (
                                <label key={key} className="api-sync-delete-row">
                                  <input
                                    type="checkbox"
                                    checked={selectedDeleteKeys.has(key)}
                                    onChange={() => toggleDeleteKey(key)}
                                  />
                                  <div className="api-sync-delete-copy">
                                    <ApiRouteLine row={row} />
                                    {row.description ? (
                                      <p className="api-sync-delete-desc">{row.description}</p>
                                    ) : null}
                                  </div>
                                </label>
                              );
                            })}
                          </div>
                        ) : null}
                      </section>
                    );
                  })}
                </div>
              </>
            )}
          </div>
        ) : null}

        {tab === "ignore" ? (
          <div className="api-sync-panel">
            {ignoreApis.length === 0 ? (
              <div className="api-sync-empty">无忽略项</div>
            ) : filteredIgnore.length === 0 ? (
              <div className="api-sync-empty">没有匹配「{query}」的忽略项</div>
            ) : (
              <>
                <p className="api-sync-hint">已在忽略列表中，本次同步不会新增或删除这些 API。</p>
                <div className="api-sync-groups">
                  {groupedIgnore.map(([group, rows]) => {
                    const collapsed = isGroupCollapsed(`ignore:${group}`, rows.length);
                    return (
                      <section key={group} className="api-sync-group">
                        <button type="button" className="api-sync-group-head" onClick={() => toggleGroupCollapse(`ignore:${group}`)}>
                          <span className="api-sync-group-chevron" data-collapsed={collapsed} aria-hidden>
                            ▾
                          </span>
                          <span className="api-sync-group-name">{group}</span>
                          <span className="api-sync-group-count">{rows.length}</span>
                        </button>
                        {!collapsed ? (
                          <div className="api-sync-group-body api-sync-group-body-readonly">
                            {rows.map((row) => (
                              <div key={syncRowKey(row)} className="api-sync-readonly-row">
                                <ApiRouteLine row={row} />
                              </div>
                            ))}
                          </div>
                        ) : null}
                      </section>
                    );
                  })}
                </div>
              </>
            )}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
