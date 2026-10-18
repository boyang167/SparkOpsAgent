"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  useK8sConfirm,
} from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import type { KubernetesCluster } from "../cluster-types";
import { formatNodeAge } from "../node-view";
import {
  formatRecordTime,
  parseRecordObjectPath,
  RECORDS_TABLE_COL_SPAN,
  resolveClusterLabel,
  shortRecordId,
  type RecordItem,
} from "../records-view";
import { clampPageToTotal } from "../workload-view";
import { RecordPlayer } from "./RecordPlayer";

type RecordsResponse = {
  items?: RecordItem[];
  total?: number;
};

type ClusterListResponse = {
  items?: KubernetesCluster[];
};

type QueryState = {
  keyword: string;
  page: number;
  pageSize: number;
};

const initialQuery = (): QueryState => ({
  keyword: "",
  page: 1,
  pageSize: 10,
});

export function RecordsPage({ canManage }: { canManage: boolean }) {
  const mountedRef = useRef(false);
  const listRequestSeq = useRef(0);

  const [query, setQuery] = useState<QueryState>(() => initialQuery());
  const [items, setItems] = useState<RecordItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [clusterNames, setClusterNames] = useState<Map<string, string>>(() => new Map());
  const [playOpen, setPlayOpen] = useState(false);
  const [playData, setPlayData] = useState("");
  const [playTitle, setPlayTitle] = useState("");
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const loadClusters = useCallback(async (signal?: AbortSignal) => {
    try {
      const data = await k8sFetch<ClusterListResponse>("/api/bff/v1/plugins/kubernetes/clusters", { signal });
      if (signal?.aborted || !mountedRef.current) return;
      const next = new Map<string, string>();
      for (const cluster of data.items ?? []) {
        if (cluster.id && cluster.name) next.set(cluster.id, cluster.name);
      }
      setClusterNames(next);
    } catch {
      if (signal?.aborted || !mountedRef.current) return;
      setClusterNames(new Map());
    }
  }, []);

  const load = useCallback(async (signal?: AbortSignal) => {
    const requestSeq = listRequestSeq.current + 1;
    listRequestSeq.current = requestSeq;
    const isCurrent = () => mountedRef.current && !signal?.aborted && requestSeq === listRequestSeq.current;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        page: String(query.page),
        pageSize: String(query.pageSize),
      });
      if (query.keyword.trim()) params.set("keyword", query.keyword.trim());
      const data = await k8sFetch<RecordsResponse>(`/api/bff/v1/plugins/kubernetes/records?${params.toString()}`, {
        signal,
      });
      if (!isCurrent()) return;
      const nextItems = data.items ?? [];
      const nextTotal = data.total ?? nextItems.length;
      const nextPage = clampPageToTotal(query.page, nextTotal, query.pageSize);
      if (nextPage !== query.page) {
        setItems([]);
        setTotal(nextTotal);
        setQuery((current) => ({ ...current, page: nextPage }));
        return;
      }
      setItems(nextItems);
      setTotal(nextTotal);
    } catch (err) {
      if (!isCurrent()) return;
      setItems([]);
      setTotal(0);
      setError(errorInfo(err, "加载终端审计失败"));
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [query.keyword, query.page, query.pageSize]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void loadClusters(controller.signal);
    return () => controller.abort();
  }, [loadClusters]);

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function playRecord(item: RecordItem) {
    setError(null);
    try {
      const detail = await k8sFetch<string>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(item.id)}`);
      const target = parseRecordObjectPath(item.objectPath);
      setPlayTitle(target.pod !== "-" ? `${target.namespace}/${target.pod}` : shortRecordId(item.id));
      setPlayData(detail);
      setPlayOpen(true);
    } catch (err) {
      setError(errorInfo(err, "加载录制失败"));
    }
  }

  async function deleteRecord(item: RecordItem) {
    if (!canManage) return;
    const ok = await ask({
      title: "删除录制",
      description: "确定要删除该条记录吗？此操作不可恢复。",
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await k8sFetch<void>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(item.id)}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除记录失败"));
    }
  }

  const hasActiveFilters = query.keyword.trim().length > 0;

  return (
    <div className="stack-md">
      <K8sPanelCard className="k8s-records-panel">
        <div className="k8s-clusters-toolbar">
          <div className="k8s-clusters-toolbar-filters">
            <label className="projects-search-wrap">
              <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M9 4a5 5 0 1 0 0 10 5 5 0 0 0 0-10Z" stroke="currentColor" strokeWidth="1.5" />
                <path d="M13 13l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
              <input
                className="projects-search-input"
                value={query.keyword}
                placeholder="搜索 Pod 名称"
                aria-label="搜索 Pod 名称"
                onChange={(e) => setQuery((current) => ({ ...current, keyword: e.target.value, page: 1 }))}
              />
            </label>
            {hasActiveFilters ? (
              <button
                type="button"
                className="btn-secondary projects-reset-btn"
                onClick={() => setQuery((current) => ({ ...current, keyword: "", page: 1 }))}
              >
                重置
              </button>
            ) : null}
          </div>
          <div className="k8s-toolbar-actions">
            {!loading && total > 0 ? (
              <span className="k8s-clusters-count" role="status">
                共 {total} 条记录
              </span>
            ) : null}
            <button
              type="button"
              className="btn-secondary k8s-clusters-refresh-btn"
              disabled={loading}
              aria-label="刷新列表"
              onClick={() => void load()}
            >
              ↻
            </button>
          </div>
        </div>

        <ApiErrorBanner error={error} />

        <div className="table-wrap k8s-records-table">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>ID</th>
                <th>集群</th>
                <th>命名空间</th>
                <th>Pod</th>
                <th>Container</th>
                <th>类型</th>
                <th>创建时间</th>
                <th className="w-28">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={RECORDS_TABLE_COL_SPAN}>
                    <K8sLoadingState compact />
                  </td>
                </tr>
              ) : null}
              {!loading &&
                items.map((item) => (
                  <RecordRow
                    key={item.id}
                    item={item}
                    clusterNames={clusterNames}
                    canManage={canManage}
                    onPlay={playRecord}
                    onDelete={deleteRecord}
                  />
                ))}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={RECORDS_TABLE_COL_SPAN}>
                    <K8sEmptyState
                      variant="empty"
                      compact
                      title="暂无终端审计记录"
                      description={
                        hasActiveFilters ? "没有符合搜索条件的记录，可调整 Pod 名称关键词。" : "当前还没有终端会话录制。"
                      }
                    />
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <K8sPagination
          page={query.page}
          pageSize={query.pageSize}
          total={total}
          loading={loading}
          onPageChange={(page) => setQuery((current) => ({ ...current, page }))}
          onPageSizeChange={(pageSize) => setQuery((current) => ({ ...current, pageSize, page: 1 }))}
        />
      </K8sPanelCard>

      <Modal
        open={playOpen}
        title={playTitle ? `回放 · ${playTitle}` : "终端录制回放"}
        size="xl"
        panelClassName="k8s-pod-submodal"
        onClose={() => setPlayOpen(false)}
      >
        {playData ? <RecordPlayer ndjson={playData} /> : null}
      </Modal>
      {confirmDialog}
    </div>
  );
}

function RecordRow({
  item,
  clusterNames,
  canManage,
  onPlay,
  onDelete,
}: {
  item: RecordItem;
  clusterNames: ReadonlyMap<string, string>;
  canManage: boolean;
  onPlay: (item: RecordItem) => Promise<void>;
  onDelete: (item: RecordItem) => Promise<void>;
}) {
  const target = parseRecordObjectPath(item.objectPath);
  const cluster = resolveClusterLabel(item.clusterId, clusterNames);
  const createdAt = formatRecordTime(item.startedAt);
  const age = formatNodeAge(item.startedAt);

  return (
    <tr>
      <td className="text-muted font-mono text-xs" title={item.id}>
        {shortRecordId(item.id)}
      </td>
      <td className={clusterNames.has(item.clusterId) ? "font-medium" : "text-muted"} title={cluster.title}>
        {cluster.label}
      </td>
      <td className="text-muted">{target.namespace}</td>
      <td className="font-medium">{target.pod}</td>
      <td className="text-muted">{target.container}</td>
      <td className="text-muted">{item.kind || "-"}</td>
      <td className="text-muted" title={createdAt}>
        {age}
      </td>
      <td>
        <div className="k8s-row-actions">
          <button type="button" className="btn-ghost text-sm" onClick={() => void onPlay(item)}>
            播放
          </button>
          {canManage ? (
            <button type="button" className="btn-ghost text-sm is-danger" onClick={() => void onDelete(item)}>
              删除
            </button>
          ) : null}
        </div>
      </td>
    </tr>
  );
}
