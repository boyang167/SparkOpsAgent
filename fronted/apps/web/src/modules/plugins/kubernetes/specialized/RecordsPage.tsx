"use client";

import { useEffect, useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";

type RecordItem = {
  id: string;
  clusterId: string;
  kind: string;
  startedAt: string;
  endedAt?: string;
  objectPath?: string;
};

type RecordsResponse = {
  items?: RecordItem[];
  total?: number;
};

export function RecordsPage({ canManage }: { canManage: boolean }) {
  const [items, setItems] = useState<RecordItem[]>([]);
  const [detail, setDetail] = useState("");
  const [error, setError] = useState<ApiErrorInfo | null>(null);

  async function load() {
    setError(null);
    try {
      const data = await k8sFetch<RecordsResponse>("/api/bff/v1/plugins/kubernetes/records?page=1&pageSize=20");
      setItems(data.items ?? []);
    } catch (err) {
      setError(errorInfo(err, "加载终端审计失败"));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function open(id: string) {
    setError(null);
    try {
      setDetail(await k8sFetch<string>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(id)}`));
    } catch (err) {
      setError(errorInfo(err, "加载记录详情失败"));
    }
  }

  async function remove(id: string) {
    if (!window.confirm("确认删除该终端审计记录？")) return;
    try {
      await k8sFetch<void>(`/api/bff/v1/plugins/kubernetes/records/${encodeURIComponent(id)}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除记录失败"));
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          刷新
        </button>
      </div>
      <ApiErrorBanner error={error} />
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>集群</th>
              <th>类型</th>
              <th>开始时间</th>
              <th>对象</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{item.id}</td>
                <td>{item.clusterId}</td>
                <td>{item.kind}</td>
                <td>{item.startedAt}</td>
                <td>{item.objectPath || "-"}</td>
                <td>
                  <div className="flex gap-2">
                    <button type="button" className="btn-secondary text-xs" onClick={() => void open(item.id)}>
                      详情
                    </button>
                    {canManage ? (
                      <button type="button" className="btn-secondary text-xs" onClick={() => void remove(item.id)}>
                        删除
                      </button>
                    ) : null}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {detail ? <pre className="input input-mono min-h-48 whitespace-pre-wrap text-xs">{detail}</pre> : null}
    </div>
  );
}
