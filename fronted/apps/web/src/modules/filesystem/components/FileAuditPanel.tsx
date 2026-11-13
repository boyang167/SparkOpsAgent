"use client";

import { useCallback, useEffect, useState } from "react";
import type { FileAuditEventVM, FileAuditQuery } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { listFileAuditEvents } from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";

export type FileAuditPanelProps = {
  workspaceId: string;
};

const PAGE_SIZE = 20;

export function FileAuditPanel({ workspaceId }: FileAuditPanelProps) {
  const [items, setItems] = useState<FileAuditEventVM[]>([]);
  const [cursor, setCursor] = useState("");
  const [nextCursor, setNextCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<FileAuditQuery>({ limit: PAGE_SIZE });

  const load = useCallback(
    async (cursorOverride?: string) => {
      setLoading(true);
      setError(null);
      try {
        const page = await listFileAuditEvents(workspaceId, {
          ...filters,
          cursor: cursorOverride ?? filters.cursor,
          limit: PAGE_SIZE,
        });
        setItems(page.items);
        setNextCursor(page.next_cursor ?? "");
        setCursor(cursorOverride ?? filters.cursor ?? "");
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载文件审计失败");
      } finally {
        setLoading(false);
      }
    },
    [filters, workspaceId],
  );

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <section className="stack-lg">
      <div>
        <h2 className="text-lg font-semibold">文件审计</h2>
        <p className="text-sm text-muted">按操作者、动作、资源、结果与时间筛选。</p>
      </div>

      <form
        className="grid gap-3 md:grid-cols-2 lg:grid-cols-3"
        onSubmit={(e) => {
          e.preventDefault();
          void load("");
        }}
      >
        <label className="stack-xs">
          <span>操作者 ID</span>
          <input
            className="input font-mono text-xs"
            value={filters.actor_id ?? ""}
            onChange={(e) => setFilters((f) => ({ ...f, actor_id: e.target.value || undefined }))}
          />
        </label>
        <label className="stack-xs">
          <span>动作</span>
          <input
            className="input"
            value={filters.action ?? ""}
            onChange={(e) => setFilters((f) => ({ ...f, action: e.target.value || undefined }))}
          />
        </label>
        <label className="stack-xs">
          <span>资源 ID</span>
          <input
            className="input font-mono text-xs"
            value={filters.resource_id ?? ""}
            onChange={(e) => setFilters((f) => ({ ...f, resource_id: e.target.value || undefined }))}
          />
        </label>
        <label className="stack-xs">
          <span>结果</span>
          <input
            className="input"
            value={filters.result ?? ""}
            onChange={(e) => setFilters((f) => ({ ...f, result: e.target.value || undefined }))}
          />
        </label>
        <label className="stack-xs">
          <span>起始时间</span>
          <input
            type="datetime-local"
            className="input"
            onChange={(e) =>
              setFilters((f) => ({
                ...f,
                from: e.target.value ? new Date(e.target.value).toISOString() : undefined,
              }))
            }
          />
        </label>
        <label className="stack-xs">
          <span>结束时间</span>
          <input
            type="datetime-local"
            className="input"
            onChange={(e) =>
              setFilters((f) => ({
                ...f,
                to: e.target.value ? new Date(e.target.value).toISOString() : undefined,
              }))
            }
          />
        </label>
        <div className="flex items-end gap-2">
          <button type="submit" className="btn btn-primary">
            查询
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={!nextCursor}
            onClick={() => void load(nextCursor)}
          >
            下一页
          </button>
        </div>
      </form>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无审计记录</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>时间</th>
                <th>操作者</th>
                <th>动作</th>
                <th>资源</th>
                <th>结果</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td className="whitespace-nowrap text-muted">
                    {new Date(item.created_at).toLocaleString()}
                  </td>
                  <td className="font-mono text-xs">{item.actor_id.slice(0, 8)}…</td>
                  <td>{item.action}</td>
                  <td className="font-mono text-xs">
                    {item.resource_type}/{item.resource_id.slice(0, 8)}…
                  </td>
                  <td>{item.result}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {cursor ? <p className="text-xs text-muted">当前游标：{cursor}</p> : null}
    </section>
  );
}
