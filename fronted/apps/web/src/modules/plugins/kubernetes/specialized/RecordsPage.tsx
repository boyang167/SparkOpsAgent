"use client";

import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
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
import {
  formatRecordTime,
  parseRecordObjectPath,
  RECORDS_TABLE_COL_SPAN,
  shortRecordId,
  type RecordItem,
} from "../records-view";
import { clampPageToTotal } from "../workload-view";
import { RecordPlayer } from "./RecordPlayer";

type RecordsResponse = {
  items?: RecordItem[];
  total?: number;
};

type QueryState = {
  draftKeyword: string;
  keyword: string;
  page: number;
  pageSize: number;
};

const initialQuery = (): QueryState => ({
  draftKeyword: "",
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
  const [selected, setSelected] = useState<string[]>([]);
  const [playOpen, setPlayOpen] = useState(false);
  const [playData, setPlayData] = useState("");
  const { ask, dialog: confirmDialog } = useK8sConfirm();

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
      setSelected([]);
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
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function playRecord(item: RecordItem) {
    setError(null);
    try {
      const detail = await k8sFetch<string>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(item.id)}`);
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

  async function deleteSelected() {
    if (!canManage || selected.length === 0) return;
    const ok = await ask({
      title: "批量删除录制",
      description: `确定要删除选中的 ${selected.length} 条记录吗？此操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      for (const id of selected) {
        await k8sFetch<void>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(id)}`, { method: "DELETE" });
      }
      setSelected([]);
      await load();
    } catch (err) {
      setError(errorInfo(err, "批量删除失败"));
    }
  }

  return (
    <div className="stack-md">
      <K8sPanelCard>
        <ApiErrorBanner error={error} />

        <div className="k8s-toolbar-row">
          <label>
            <span className="label">Pod 名称</span>
            <input
              className="input"
              placeholder="Pod 名称"
              value={query.draftKeyword}
              onChange={(e) => setQuery((current) => ({ ...current, draftKeyword: e.target.value }))}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  setQuery((current) => ({ ...current, keyword: current.draftKeyword, page: 1 }));
                }
              }}
            />
          </label>
          <button
            type="button"
            className="btn"
            disabled={loading}
            onClick={() => setQuery((current) => ({ ...current, keyword: current.draftKeyword, page: 1 }))}
          >
            查询
          </button>
          <button
            type="button"
            className="btn-secondary"
            disabled={loading}
            onClick={() => setQuery({ draftKeyword: "", keyword: "", page: 1, pageSize: query.pageSize })}
          >
            重置
          </button>
          <button type="button" className="btn-secondary" disabled={!canManage || selected.length === 0} onClick={() => void deleteSelected()}>
            删除
          </button>
        </div>

        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th className="w-10">
                  <span className="sr-only">选择</span>
                </th>
                <th>ID</th>
                <th>集群</th>
                <th>命名空间</th>
                <th>Pod</th>
                <th>Container</th>
                <th>类型</th>
                <th>创建时间</th>
                <th>操作</th>
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
                    canManage={canManage}
                    selected={selected}
                    setSelected={setSelected}
                    onPlay={playRecord}
                    onDelete={deleteRecord}
                  />
                ))}
              {!loading && items.length === 0 ? (
                <tr>
                  <td colSpan={RECORDS_TABLE_COL_SPAN}>
                    <K8sEmptyState variant="empty" compact />
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

      <Modal open={playOpen} title="终端录制播放" size="xl" onClose={() => setPlayOpen(false)}>
        {playData ? <RecordPlayer ndjson={playData} /> : null}
      </Modal>
      {confirmDialog}
    </div>
  );
}

function RecordRow({
  item,
  canManage,
  selected,
  setSelected,
  onPlay,
  onDelete,
}: {
  item: RecordItem;
  canManage: boolean;
  selected: string[];
  setSelected: Dispatch<SetStateAction<string[]>>;
  onPlay: (item: RecordItem) => Promise<void>;
  onDelete: (item: RecordItem) => Promise<void>;
}) {
  const target = parseRecordObjectPath(item.objectPath);

  return (
    <tr>
      <td>
        <input
          type="checkbox"
          checked={selected.includes(item.id)}
          onChange={(e) =>
            setSelected((current) => (e.target.checked ? [...current, item.id] : current.filter((value) => value !== item.id)))
          }
        />
      </td>
      <td title={item.id}>{shortRecordId(item.id)}</td>
      <td>{item.clusterId || "-"}</td>
      <td>{target.namespace}</td>
      <td>{target.pod}</td>
      <td>{target.container}</td>
      <td>{item.kind || "-"}</td>
      <td>{formatRecordTime(item.startedAt)}</td>
      <td className="whitespace-nowrap">
        <button type="button" className="k8s-action-link" onClick={() => void onPlay(item)}>
          播放
        </button>
        {canManage ? (
          <>
            <span className="text-muted"> / </span>
            <button type="button" className="k8s-action-link" onClick={() => void onDelete(item)}>
              删除
            </button>
          </>
        ) : null}
      </td>
    </tr>
  );
}
