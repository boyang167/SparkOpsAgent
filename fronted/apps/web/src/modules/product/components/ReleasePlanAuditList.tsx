"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import {
  formatAuditDetail,
  releasePlanAuditActionLabel,
} from "./release-plan-utils";

export interface ReleasePlanAuditLogEntry {
  id?: string;
  action?: string;
  actor_id?: string;
  detail?: unknown;
  created_at?: string;
}

interface ReleasePlanAuditListProps {
  workspaceId: string;
  productId: string;
  planId: string;
  memberName: (memberId?: string | null) => string;
}

function hasDetailPayload(detail: unknown): boolean {
  if (detail == null) return false;
  if (typeof detail === "object" && !Array.isArray(detail)) {
    return Object.keys(detail as object).length > 0;
  }
  return true;
}

export function ReleasePlanAuditList({ workspaceId, productId, planId, memberName }: ReleasePlanAuditListProps) {
  const [logs, setLogs] = useState<ReleasePlanAuditLogEntry[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionFilter, setActionFilter] = useState("");
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await product.listReleasePlanAuditLogs(workspaceId, productId, planId);
      setLogs(page.items);
      setHasMore(page.has_more);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载操作记录失败");
      setLogs([]);
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId, planId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function loadMore() {
    setLoadingMore(true);
    try {
      const page = await product.listReleasePlanAuditLogs(workspaceId, productId, planId, logs.length);
      setLogs((prev) => [...prev, ...page.items]);
      setHasMore(page.has_more);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载更多操作记录失败");
    } finally {
      setLoadingMore(false);
    }
  }

  const actionOptions = useMemo(() => {
    const actions = [...new Set(logs.map((log) => String(log.action ?? "")).filter(Boolean))];
    actions.sort((a, b) => releasePlanAuditActionLabel(a).localeCompare(releasePlanAuditActionLabel(b), "zh-CN"));
    return actions;
  }, [logs]);

  const filteredLogs = useMemo(() => {
    if (!actionFilter) return logs;
    return logs.filter((log) => String(log.action ?? "") === actionFilter);
  }, [logs, actionFilter]);

  function toggleExpanded(key: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  if (loading) {
    return <p className="release-plan-audit-empty">加载操作记录中…</p>;
  }

  if (logs.length === 0) {
    return <p className="release-plan-audit-empty">暂无操作记录</p>;
  }

  return (
    <div className="release-plan-audit-panel stack-sm">
      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {actionOptions.length > 1 ? (
        <div className="release-plan-audit-toolbar">
          <label className="release-plan-audit-filter">
            <span className="release-plan-audit-filter-label">操作类型</span>
            <select
              className="input text-xs"
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value)}
              aria-label="筛选操作类型"
            >
              <option value="">全部</option>
              {actionOptions.map((action) => (
                <option key={action} value={action}>
                  {releasePlanAuditActionLabel(action)}
                </option>
              ))}
            </select>
          </label>
          <span className="release-plan-audit-count text-xs text-muted-foreground">
            {filteredLogs.length} / {logs.length} 条
          </span>
        </div>
      ) : null}

      {filteredLogs.length === 0 ? (
        <p className="release-plan-audit-empty">没有匹配的操作记录</p>
      ) : (
        <ul className="release-plan-audit-list">
          {filteredLogs.map((log, index) => {
            const action = String(log.action ?? "-");
            const detail = formatAuditDetail(log.detail);
            const actorId = typeof log.actor_id === "string" ? log.actor_id : undefined;
            const actorLabel = actorId ? memberName(actorId) : "系统";
            const itemKey = String(log.id ?? index);
            const showRaw = hasDetailPayload(log.detail);
            const rawExpanded = expandedIds.has(itemKey);

            return (
              <li key={itemKey} className="release-plan-audit-item">
                <div className="release-plan-audit-main">
                  <span className="release-plan-audit-action">{releasePlanAuditActionLabel(action)}</span>
                  {detail ? <p className="release-plan-audit-detail">{detail}</p> : null}
                  {showRaw ? (
                    <div className="release-plan-audit-raw">
                      <button
                        type="button"
                        className="btn-ghost text-xs px-0 release-plan-audit-raw-toggle"
                        onClick={() => toggleExpanded(itemKey)}
                      >
                        {rawExpanded ? "收起原始数据" : "查看原始数据"}
                      </button>
                      {rawExpanded ? (
                        <pre className="release-plan-audit-raw-json">
                          {JSON.stringify(log.detail, null, 2)}
                        </pre>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <div className="release-plan-audit-meta">
                  <span className="release-plan-audit-actor">{actorLabel}</span>
                  {log.created_at ? (
                    <time className="release-plan-audit-time" dateTime={String(log.created_at)}>
                      {new Date(String(log.created_at)).toLocaleString("zh-CN")}
                    </time>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {hasMore ? (
        <button type="button" className="btn btn-secondary text-xs self-center" disabled={loadingMore} onClick={() => void loadMore()}>
          {loadingMore ? "加载中…" : "加载更多"}
        </button>
      ) : null}
    </div>
  );
}
