"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { secret } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateSecretCommand, SecretVM, UpdateSecretCommand } from "@sparkops/contracts/secret";
import { serviceTypeLabel } from "@sparkops/contracts/secret";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ConnectionStatusCell } from "./secret/ConnectionStatusCell";
import { ConnectionIcon, PlusIcon, RefreshIcon, SearchIcon } from "./secret/SecretIcons";
import { SecretActionGrid } from "./secret/SecretActionGrid";
import { SecretDetail } from "./secret/SecretDetail";
import { SecretForm } from "./secret/SecretForm";
import {
  creatorDisplayName,
  formatDateTime,
  labelOf,
  PAGE_SIZES,
  SECRET_SERVICE_TYPES,
  SECRET_STATUS_OPTIONS,
  serviceTypeBadgeClass,
  statusBadgeClass,
  type ConnectionStatus,
} from "./secret/secret-utils";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface SecretPanelProps {
  workspaceId: string;
}

const EMPTY_FORM: CreateSecretCommand = {
  name: "",
  display_name: "",
  description: "",
  service_type: "GitLab",
  service_url: "",
  username: "",
  credentials: {},
  status: "active",
  is_default: false,
  is_shared: false,
};

function stripEmptyCredentials(credentials: CreateSecretCommand["credentials"]) {
  if (!credentials) return {};
  const next: NonNullable<CreateSecretCommand["credentials"]> = {};
  for (const [key, value] of Object.entries(credentials)) {
    if (typeof value === "string" && value.trim()) {
      next[key as keyof typeof next] = value;
    }
  }
  return next;
}

export function SecretPanel({ workspaceId }: SecretPanelProps) {
  const keywordRef = useRef<HTMLInputElement>(null);
  const firstLoadRef = useRef(true);
  const [items, setItems] = useState<SecretVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [keyword, setKeyword] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [connectionMap, setConnectionMap] = useState<Record<string, ConnectionStatus>>({});
  const [bulkTesting, setBulkTesting] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [editing, setEditing] = useState<SecretVM | null>(null);
  const [selected, setSelected] = useState<SecretVM | null>(null);
  const [form, setForm] = useState<CreateSecretCommand>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const { confirm, ConfirmDialog } = useConfirmDialog();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await secret.listSecrets(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载密钥配置失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  const testSingle = useCallback(
    async (item: SecretVM, showToast = true) => {
      setConnectionMap((prev) => ({ ...prev, [item.id]: "testing" }));
      try {
        const result = await secret.testSecretConnection(workspaceId, item.id);
        setConnectionMap((prev) => ({
          ...prev,
          [item.id]: result.success ? "success" : "failed",
        }));
        if (showToast) {
          if (result.success) setSuccess(result.message || "连接测试成功");
          else setError(result.message || "连接测试失败");
        }
        return result;
      } catch (err) {
        setConnectionMap((prev) => ({ ...prev, [item.id]: "failed" }));
        if (showToast) {
          setError(err instanceof BffError ? err.message : "连接测试失败");
        }
        throw err;
      }
    },
    [workspaceId],
  );

  const testAllActive = useCallback(
    async (list: SecretVM[]) => {
      const active = list.filter((item) => item.status === "active");
      if (active.length === 0) return;
      setBulkTesting(true);
      const concurrency = 3;
      for (let i = 0; i < active.length; i += concurrency) {
        const batch = active.slice(i, i + concurrency);
        await Promise.allSettled(batch.map((item) => testSingle(item, false)));
        if (i + concurrency < active.length) {
          await new Promise((r) => setTimeout(r, 200));
        }
      }
      setBulkTesting(false);
    },
    [testSingle],
  );

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  useEffect(() => {
    if (loading || items.length === 0 || !firstLoadRef.current) return;
    firstLoadRef.current = false;
    const timer = window.setTimeout(() => {
      void testAllActive(items);
    }, 500);
    return () => window.clearTimeout(timer);
  }, [loading, items, testAllActive]);

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
    const kw = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (typeFilter && item.service_type !== typeFilter) return false;
      if (statusFilter && item.status !== statusFilter) return false;
      if (!kw) return true;
      return [item.name, item.display_name, item.service_url, item.username, item.description]
        .some((v) => v?.toLowerCase().includes(kw));
    });
  }, [items, keyword, typeFilter, statusFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filtered.slice(start, start + pageSize);
  }, [filtered, page, pageSize]);

  const allPageSelected = paged.length > 0 && paged.every((item) => selectedIds.has(item.id));

  useEffect(() => {
    setPage(1);
  }, [keyword, typeFilter, statusFilter, pageSize]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  function resetFilters() {
    setKeyword("");
    setTypeFilter("");
    setStatusFilter("");
  }

  function toggleAllPage(checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      for (const item of paged) {
        if (checked) next.add(item.id);
        else next.delete(item.id);
      }
      return next;
    });
  }

  function toggleRow(id: string, checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function openCreate() {
    setEditing(null);
    setForm(EMPTY_FORM);
    setFormOpen(true);
  }

  function openEdit(item: SecretVM) {
    setEditing(item);
    setForm({
      name: item.name,
      display_name: item.display_name ?? "",
      description: item.description ?? "",
      service_type: item.service_type,
      service_url: item.service_url,
      username: item.username ?? "",
      credentials: {},
      status: item.status ?? "active",
      is_default: item.is_default ?? false,
      is_shared: item.is_shared ?? false,
    });
    setFormOpen(true);
  }

  function openDetail(item: SecretVM) {
    setSelected(item);
    setDetailOpen(true);
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      const credentials = stripEmptyCredentials(form.credentials);
      if (editing) {
        const payload: UpdateSecretCommand = {
          display_name: form.display_name,
          description: form.description,
          service_type: form.service_type,
          service_url: form.service_url,
          username: form.username,
          credentials,
          status: form.status,
          is_default: form.is_default,
          is_shared: form.is_shared,
        };
        await secret.updateSecret(workspaceId, editing.id, payload);
        setSuccess("密钥配置已更新");
      } else {
        const created = await secret.createSecret(workspaceId, { ...form, credentials });
        setSuccess("密钥配置已创建");
        void testSingle(created, false);
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function onDelete(item: SecretVM) {
    const ok = await confirm({ title: `确定删除配置「${item.name}」？此操作不可恢复。`, danger: true });
    if (!ok) return;
    setError(null);
    setSuccess(null);
    try {
      await secret.deleteSecret(workspaceId, item.id);
      if (selected?.id === item.id) setDetailOpen(false);
      setSelectedIds((prev) => {
        const next = new Set(prev);
        next.delete(item.id);
        return next;
      });
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    }
  }

  async function onBatchDelete() {
    if (selectedIds.size === 0) return;
    const ok = await confirm({ title: `确定删除选中的 ${selectedIds.size} 个配置？`, danger: true });
    if (!ok) return;
    setError(null);
    try {
      await Promise.all([...selectedIds].map((id) => secret.deleteSecret(workspaceId, id)));
      setSelectedIds(new Set());
      setDetailOpen(false);
      await load();
      setSuccess("批量删除成功");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "批量删除失败");
    }
  }

  async function onTest(item: SecretVM) {
    setTestingId(item.id);
    setError(null);
    setSuccess(null);
    try {
      await testSingle(item, true);
    } finally {
      setTestingId(null);
    }
  }

  async function handleRefresh() {
    await load();
    void testAllActive(items);
  }

  return (
    <>
    <div className="secret-panel">
      <div className="secret-search-section">
        <div className="secret-search-form">
          <label className="secret-inline-field">
            <span>配置名称</span>
            <input
              ref={keywordRef}
              className="input"
              placeholder="请输入配置名称"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
          </label>
          <label className="secret-inline-field">
            <span>服务类型</span>
            <select className="input" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
              <option value="">请选择服务类型</option>
              {SECRET_SERVICE_TYPES.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <label className="secret-inline-field">
            <span>状态</span>
            <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="">请选择状态</option>
              {SECRET_STATUS_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <div className="secret-search-actions">
            <button type="button" className="btn secret-btn-primary">
              <SearchIcon />
              搜索
            </button>
            <button type="button" className="btn-secondary" onClick={resetFilters}>
              <RefreshIcon />
              重置
            </button>
          </div>
        </div>
        <div className="secret-toolbar-actions">
          <button type="button" className="btn secret-btn-primary" onClick={openCreate}>
            <PlusIcon />
            创建配置
          </button>
          <button type="button" className="btn-secondary" onClick={() => void handleRefresh()} disabled={loading}>
            <RefreshIcon />
            刷新
          </button>
        </div>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="info">{success}</InlineAlert> : null}

      <div className="secret-table-section">
        {loading ? (
          <div className="secret-table-loading">加载中…</div>
        ) : filtered.length === 0 ? (
          <div className="secret-empty-state">
            <p>暂无密钥配置</p>
            <button type="button" className="btn secret-btn-primary" onClick={openCreate}>
              <PlusIcon />
              创建第一个配置
            </button>
          </div>
        ) : (
          <>
            <div className="table-wrap">
              <table className="data-table secret-table">
                <thead>
                  <tr>
                    <th className="secret-col-check">
                      <input
                        type="checkbox"
                        checked={allPageSelected}
                        onChange={(e) => toggleAllPage(e.target.checked)}
                        aria-label="全选当前页"
                      />
                    </th>
                    <th>配置名称</th>
                    <th>服务类型</th>
                    <th>服务地址</th>
                    <th>用户名</th>
                    <th>状态</th>
                    <th>连接状态</th>
                    <th>默认</th>
                    <th>共享</th>
                    <th>创建者</th>
                    <th>创建时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paged.map((item) => (
                    <tr key={item.id} className={selectedIds.has(item.id) ? "is-selected" : ""}>
                      <td className="secret-col-check">
                        <input
                          type="checkbox"
                          checked={selectedIds.has(item.id)}
                          onChange={(e) => toggleRow(item.id, e.target.checked)}
                          aria-label={`选择 ${item.name}`}
                        />
                      </td>
                      <td>
                        <div className="secret-name-cell">
                          <ConnectionIcon />
                          <div>
                            <div className="secret-name-primary">{item.name}</div>
                            {item.display_name ? (
                              <div className="secret-name-secondary">{item.display_name}</div>
                            ) : null}
                          </div>
                        </div>
                      </td>
                      <td>
                        <span className={serviceTypeBadgeClass(item.service_type)}>
                          {serviceTypeLabel(item.service_type)}
                        </span>
                      </td>
                      <td className="secret-url-cell" title={item.service_url}>
                        {item.service_url}
                      </td>
                      <td>{item.username || "-"}</td>
                      <td>
                        <span className={statusBadgeClass(item.status)}>
                          {labelOf(SECRET_STATUS_OPTIONS, item.status)}
                        </span>
                      </td>
                      <td>
                        <ConnectionStatusCell
                          status={connectionMap[item.id]}
                          bulkTesting={bulkTesting}
                          isActive={item.status === "active"}
                        />
                      </td>
                      <td>
                        {item.is_default ? (
                          <span className="secret-tag secret-tag-orange">默认</span>
                        ) : (
                          <span className="secret-muted">-</span>
                        )}
                      </td>
                      <td>
                        {item.is_shared ? (
                          <span className="secret-tag secret-tag-green">共享</span>
                        ) : (
                          <span className="secret-muted">-</span>
                        )}
                      </td>
                      <td>{creatorDisplayName(item)}</td>
                      <td>{formatDateTime(item.created_at)}</td>
                      <td>
                        <SecretActionGrid
                          testing={testingId === item.id}
                          onView={() => openDetail(item)}
                          onEdit={() => openEdit(item)}
                          onTest={() => void onTest(item)}
                          onDelete={() => void onDelete(item)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="secret-pagination">
              <span className="secret-pagination-total">共 {filtered.length} 条</span>
              <select
                className="input secret-page-size"
                value={pageSize}
                onChange={(e) => setPageSize(Number(e.target.value))}
              >
                {PAGE_SIZES.map((size) => (
                  <option key={size} value={size}>
                    {size} 条/页
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn-secondary secret-page-btn"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                上一页
              </button>
              <span className="secret-page-indicator">
                {page} / {totalPages}
              </span>
              <button
                type="button"
                className="btn-secondary secret-page-btn"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                下一页
              </button>
            </div>
          </>
        )}
      </div>

      {selectedIds.size > 0 ? (
        <div className="secret-batch-bar">
          <span>已选择 {selectedIds.size} 个配置</span>
          <button type="button" className="secret-action-btn is-delete" onClick={() => void onBatchDelete()}>
            批量删除
          </button>
        </div>
      ) : null}

      <Modal
        open={formOpen}
        onClose={() => setFormOpen(false)}
        title={editing ? "编辑服务连接配置" : "创建服务连接配置"}
        size="lg"
      >
        <SecretForm
          form={form}
          isEdit={Boolean(editing)}
          submitting={submitting}
          onChange={setForm}
          onSubmit={onSubmit}
          onCancel={() => setFormOpen(false)}
        />
      </Modal>

      <Modal
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        title="服务连接配置详情"
        size="xl"
        footer={
          <div className="secret-modal-footer">
            <button type="button" className="btn-secondary" onClick={() => setDetailOpen(false)}>
              关闭
            </button>
            <button
              type="button"
              className="btn secret-btn-primary"
              onClick={() => {
                if (selected) {
                  setDetailOpen(false);
                  openEdit(selected);
                }
              }}
            >
              编辑
            </button>
          </div>
        }
      >
        {selected ? (
          <SecretDetail
            secret={selected}
            testing={testingId === selected.id}
            onTest={async () => {
              setTestingId(selected.id);
              try {
                return await testSingle(selected, false);
              } finally {
                setTestingId(null);
              }
            }}
          />
        ) : null}
      </Modal>
    </div>
    <ConfirmDialog />
    </>
  );
}
