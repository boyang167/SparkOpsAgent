"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import type { ListOperationRecordsQuery, OperationRecordVM } from "@sparkops/contracts/platform";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;
const PAGE_SIZES = [10, 20, 50, 100] as const;

function fmtBody(value: string) {
  if (!value) return "无";
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function operatorLabel(memberId: string | undefined, memberMap: Map<string, WorkspaceMemberVM>) {
  if (!memberId) return "—";
  const member = memberMap.get(memberId);
  if (!member) return `${memberId.slice(0, 8)}…`;
  const username = member.email.includes("@") ? member.email.split("@")[0] : member.email;
  const nick = member.display_name?.trim();
  return nick ? `${username}(${nick})` : username;
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function statusTagClass(status: number) {
  if (status >= 200 && status < 300) return "op-log-status op-log-status--ok";
  if (status >= 400 && status < 500) return "op-log-status op-log-status--warn";
  if (status >= 500) return "op-log-status op-log-status--error";
  return "op-log-status";
}

function buildPageList(current: number, total: number) {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set<number>([1, total, current, current - 1, current + 1]);
  const sorted = [...pages].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);
  const out: Array<number | "ellipsis"> = [];
  for (let i = 0; i < sorted.length; i++) {
    if (i > 0 && sorted[i] - sorted[i - 1] > 1) out.push("ellipsis");
    out.push(sorted[i]);
  }
  return out;
}

interface OperationRecordsPanelProps {
  workspaceId: string;
  members?: WorkspaceMemberVM[];
}

export function OperationRecordsPanel({ workspaceId, members = [] }: OperationRecordsPanelProps) {
  const [items, setItems] = useState<OperationRecordVM[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [detailRow, setDetailRow] = useState<OperationRecordVM | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const [jumpPage, setJumpPage] = useState("");

  const [filters, setFilters] = useState<ListOperationRecordsQuery>({
    page: 1,
    page_size: PAGE_SIZES[0],
  });
  const [applied, setApplied] = useState(filters);

  const memberMap = useMemo(
    () => new Map(members.map((member) => [member.member_id, member])),
    [members],
  );

  const pageSize = applied.page_size ?? PAGE_SIZES[0];
  const currentPage = applied.page ?? 1;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const allPageSelected = items.length > 0 && items.every((row) => selectedIds.has(row.id));

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await platform.listOperationRecords(workspaceId, applied);
      setItems(data.items);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, applied]);

  useEffect(() => {
    void load();
  }, [load]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3200);
  }

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setApplied({ ...filters, page: 1 });
    setSelectedIds(new Set());
  }

  function onReset() {
    const next = { page: 1, page_size: PAGE_SIZES[0] as number };
    setFilters(next);
    setApplied(next);
    setSelectedIds(new Set());
  }

  function goPage(page: number) {
    const next = Math.min(totalPages, Math.max(1, page));
    setApplied((prev) => ({ ...prev, page: next }));
    setFilters((prev) => ({ ...prev, page: next }));
  }

  function onJumpPage(e: React.FormEvent) {
    e.preventDefault();
    const page = Number(jumpPage);
    if (!Number.isFinite(page) || page < 1) return;
    goPage(page);
    setJumpPage("");
  }

  function toggleSelectAllPage() {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (allPageSelected) {
        for (const row of items) next.delete(row.id);
      } else {
        for (const row of items) next.add(row.id);
      }
      return next;
    });
  }

  async function confirmBatchDelete() {
    setActionId("batch");
    try {
      const result = await platform.batchDeleteOperationRecords(workspaceId, {
        ids: Array.from(selectedIds),
      });
      setBatchDeleteOpen(false);
      setSelectedIds(new Set());
      flashSuccess(`已删除 ${result.deleted} 条记录`);
      if (items.length === selectedIds.size && currentPage > 1) {
        goPage(currentPage - 1);
      } else {
        await load();
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "批量删除失败");
    } finally {
      setActionId(null);
    }
  }

  const pageList = buildPageList(currentPage, totalPages);

  return (
    <div className="op-log-page">
      <form onSubmit={onSearch} className="op-log-search panel">
        <label className="op-log-search-field">
          <span className="op-log-search-label">请求方法</span>
          <select
            className="input"
            value={filters.method ?? ""}
            onChange={(e) => setFilters((prev) => ({ ...prev, method: e.target.value || undefined }))}
          >
            <option value="">搜索条件</option>
            {HTTP_METHODS.map((method) => (
              <option key={method} value={method}>
                {method}
              </option>
            ))}
          </select>
        </label>
        <label className="op-log-search-field">
          <span className="op-log-search-label">请求路径</span>
          <input
            className="input"
            placeholder="搜索条件"
            value={filters.path ?? ""}
            onChange={(e) => setFilters((prev) => ({ ...prev, path: e.target.value || undefined }))}
          />
        </label>
        <label className="op-log-search-field">
          <span className="op-log-search-label">结果状态码</span>
          <input
            className="input"
            placeholder="搜索条件"
            value={filters.status != null ? String(filters.status) : ""}
            onChange={(e) =>
              setFilters((prev) => ({
                ...prev,
                status: e.target.value ? Number(e.target.value) : undefined,
              }))
            }
          />
        </label>
        <div className="op-log-search-actions">
          <button type="submit" className="btn">
            查询
          </button>
          <button type="button" className="btn-secondary" onClick={onReset}>
            重置
          </button>
        </div>
      </form>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <div className="op-log-table-panel panel">
        <div className="op-log-toolbar">
          <button
            type="button"
            className="op-log-delete-btn"
            disabled={selectedIds.size === 0 || actionId === "batch"}
            onClick={() => setBatchDeleteOpen(true)}
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            删除
          </button>
        </div>

        <div className="table-wrap op-log-table-wrap">
          <table className="data-table op-log-table text-sm">
            <thead>
              <tr>
                <th className="w-10">
                  <input type="checkbox" checked={allPageSelected} onChange={toggleSelectAllPage} />
                </th>
                <th>操作人</th>
                <th>日期</th>
                <th>状态码</th>
                <th>请求IP</th>
                <th>请求ID</th>
                <th>链路ID</th>
                <th>设备ID</th>
                <th>请求方法</th>
                <th>请求路径</th>
                <th className="w-12" />
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={11} className="op-log-empty">
                    加载中…
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={11} className="op-log-empty">
                    暂无操作日志，发起 API 请求后会自动采集
                  </td>
                </tr>
              ) : (
                items.map((row) => (
                  <tr key={row.id}>
                    <td>
                      <input
                        type="checkbox"
                        checked={selectedIds.has(row.id)}
                        onChange={() =>
                          setSelectedIds((prev) => {
                            const next = new Set(prev);
                            if (next.has(row.id)) next.delete(row.id);
                            else next.add(row.id);
                            return next;
                          })
                        }
                      />
                    </td>
                    <td className="op-log-operator">{operatorLabel(row.member_id, memberMap)}</td>
                    <td className="whitespace-nowrap">{formatDate(row.created_at)}</td>
                    <td>
                      <span className={statusTagClass(row.status)}>{row.status}</span>
                    </td>
                    <td>{row.ip || "—"}</td>
                    <td className="op-log-mono" title={row.id}>
                      {row.id}
                    </td>
                    <td className="op-log-mono" title={row.trace_id}>
                      {row.trace_id || "—"}
                    </td>
                    <td>无</td>
                    <td>{row.method}</td>
                    <td className="op-log-path" title={row.path}>
                      {row.path}
                    </td>
                    <td>
                      <button
                        type="button"
                        className="op-log-info-btn"
                        aria-label="查看详情"
                        onClick={() => setDetailRow(row)}
                      >
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                          <circle cx="12" cy="12" r="9" />
                          <path d="M12 10v6M12 7h.01" strokeLinecap="round" />
                        </svg>
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="op-log-pagination">
          <span className="op-log-pagination-total">共 {total} 条</span>
          <select
            className="input op-log-page-size"
            value={pageSize}
            onChange={(e) => {
              const nextSize = Number(e.target.value);
              setApplied((prev) => ({ ...prev, page_size: nextSize, page: 1 }));
              setFilters((prev) => ({ ...prev, page_size: nextSize, page: 1 }));
            }}
          >
            {PAGE_SIZES.map((size) => (
              <option key={size} value={size}>
                {size}条/页
              </option>
            ))}
          </select>
          <div className="op-log-page-list">
            <button
              type="button"
              className="op-log-page-btn"
              disabled={currentPage <= 1}
              onClick={() => goPage(currentPage - 1)}
              aria-label="上一页"
            >
              ‹
            </button>
            {pageList.map((item, idx) =>
              item === "ellipsis" ? (
                <span key={`e-${idx}`} className="op-log-page-ellipsis">
                  …
                </span>
              ) : (
                <button
                  key={item}
                  type="button"
                  className={item === currentPage ? "op-log-page-btn is-active" : "op-log-page-btn"}
                  onClick={() => goPage(item)}
                >
                  {item}
                </button>
              ),
            )}
            <button
              type="button"
              className="op-log-page-btn"
              disabled={currentPage >= totalPages}
              onClick={() => goPage(currentPage + 1)}
              aria-label="下一页"
            >
              ›
            </button>
          </div>
          <form className="op-log-jump" onSubmit={onJumpPage}>
            <span>前往</span>
            <input
              className="input op-log-jump-input"
              value={jumpPage}
              onChange={(e) => setJumpPage(e.target.value)}
              inputMode="numeric"
            />
            <span>页</span>
          </form>
        </div>
      </div>

      <Drawer
        open={detailRow != null}
        onClose={() => setDetailRow(null)}
        title="操作详情"
        description={detailRow ? `${detailRow.method} ${detailRow.path}` : undefined}
      >
        {detailRow ? (
          <div className="op-log-detail stack-md">
            <dl className="op-log-detail-meta">
              <div>
                <dt>操作人</dt>
                <dd>{operatorLabel(detailRow.member_id, memberMap)}</dd>
              </div>
              <div>
                <dt>日期</dt>
                <dd>{formatDate(detailRow.created_at)}</dd>
              </div>
              <div>
                <dt>状态码</dt>
                <dd>
                  <span className={statusTagClass(detailRow.status)}>{detailRow.status}</span>
                </dd>
              </div>
              <div>
                <dt>延迟</dt>
                <dd>{detailRow.latency_ms} ms</dd>
              </div>
              <div>
                <dt>请求 IP</dt>
                <dd>{detailRow.ip || "—"}</dd>
              </div>
              <div>
                <dt>Agent</dt>
                <dd className="break-all">{detailRow.agent || "—"}</dd>
              </div>
              <div>
                <dt>请求 ID</dt>
                <dd className="op-log-mono">{detailRow.id}</dd>
              </div>
              <div>
                <dt>链路 ID</dt>
                <dd className="op-log-mono">
                  {detailRow.trace_id ? (
                    <Link
                      href={`/monitor/log-viewer?trace=${encodeURIComponent(detailRow.trace_id)}`}
                      className="link"
                    >
                      {detailRow.trace_id}
                    </Link>
                  ) : (
                    "—"
                  )}
                </dd>
              </div>
            </dl>

            <div>
              <p className="op-log-detail-label">请求 Body</p>
              <pre className="op-log-code-block">{fmtBody(detailRow.body)}</pre>
            </div>
            <div>
              <p className="op-log-detail-label">响应 Body</p>
              <pre className="op-log-code-block">{fmtBody(detailRow.resp)}</pre>
            </div>
            {detailRow.error_message ? (
              <div>
                <p className="op-log-detail-label">错误信息</p>
                <pre className="op-log-error">{detailRow.error_message}</pre>
              </div>
            ) : null}
          </div>
        ) : null}
      </Drawer>

      <Modal open={batchDeleteOpen} onClose={() => setBatchDeleteOpen(false)} title="删除确认">
        <p className="text-sm text-muted">确定要删除选中的 {selectedIds.size} 条操作日志吗？</p>
        <div className="flex justify-end gap-2 mt-4">
          <button type="button" className="btn-secondary" onClick={() => setBatchDeleteOpen(false)}>
            取消
          </button>
          <button
            type="button"
            className="btn"
            disabled={actionId === "batch"}
            onClick={() => void confirmBatchDelete()}
          >
            确定
          </button>
        </div>
      </Modal>
    </div>
  );
}
