"use client";

import type { RegisteredAPI } from "@sparkops/contracts/governance";
import { Modal } from "@/components/ui/Modal";
import { methodBadgeClass } from "./api-utils";

export type SyncRow = RegisteredAPI & { api_group?: string; description?: string };

interface ApiSyncModalProps {
  open: boolean;
  loading: boolean;
  newApis: SyncRow[];
  deleteApis: SyncRow[];
  ignoreApis: SyncRow[];
  onClose: () => void;
  onConfirm: () => void;
  onChangeNewApi: (index: number, patch: Partial<SyncRow>) => void;
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
}: ApiSyncModalProps) {
  return (
    <Modal open={open} onClose={onClose} title="同步 API" description="扫描后端路由并与注册表对比差异">
      <div className="stack-md max-h-[70vh] overflow-y-auto">
        <section>
          <h3 className="font-medium text-emerald-700">新增 ({newApis.length})</h3>
          {newApis.length === 0 ? (
            <p className="text-sm text-muted mt-1">无新增 API</p>
          ) : (
            <div className="stack-sm mt-2">
              {newApis.map((row, index) => (
                <div key={`${row.method}:${row.path}`} className="rounded border p-3 stack-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`badge ${methodBadgeClass(row.method)}`}>{row.method}</span>
                    <span className="font-mono text-xs break-all">{row.path}</span>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-3">
                    <input
                      className="input"
                      placeholder="分组"
                      value={row.api_group ?? ""}
                      onChange={(e) => onChangeNewApi(index, { api_group: e.target.value })}
                    />
                    <input
                      className="input sm:col-span-2"
                      placeholder="描述"
                      value={row.description ?? ""}
                      onChange={(e) => onChangeNewApi(index, { description: e.target.value })}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section>
          <h3 className="font-medium text-red-700">删除 ({deleteApis.length})</h3>
          {deleteApis.length === 0 ? (
            <p className="text-sm text-muted mt-1">无待删除 API</p>
          ) : (
            <ul className="text-xs font-mono text-muted space-y-1 mt-2">
              {deleteApis.map((row) => (
                <li key={`${row.method}:${row.path}`}>
                  {row.method} {row.path}
                </li>
              ))}
            </ul>
          )}
        </section>

        {ignoreApis.length > 0 ? (
          <section>
            <h3 className="font-medium text-amber-700">忽略 ({ignoreApis.length})</h3>
            <p className="text-xs text-muted mt-1">已在忽略列表中，不参与同步变更</p>
            <ul className="text-xs font-mono text-muted space-y-1 mt-2">
              {ignoreApis.map((row) => (
                <li key={`${row.method}:${row.path}`}>
                  {row.method} {row.path}
                </li>
              ))}
            </ul>
          </section>
        ) : null}

        <div className="flex justify-end gap-2 pt-2 border-t border-[var(--border)]">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn" disabled={loading} onClick={onConfirm}>
            {loading ? "同步中…" : "确认同步"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
