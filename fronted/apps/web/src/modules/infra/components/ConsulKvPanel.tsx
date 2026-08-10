"use client";

import { useCallback, useEffect, useState } from "react";
import { infra } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface ConsulKvPanelProps {
  workspaceId: string;
}

type KVEntry = { key: string; value: string; updated_at?: string };

export function ConsulKvPanel({ workspaceId }: ConsulKvPanelProps) {
  const [prefix, setPrefix] = useState("");
  const [items, setItems] = useState<KVEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [editOpen, setEditOpen] = useState(false);
  const [editExisting, setEditExisting] = useState(false);
  const [editKey, setEditKey] = useState("");
  const [editValue, setEditValue] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await infra.listConsulKV(workspaceId, prefix || undefined));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, prefix]);

  useEffect(() => {
    void load();
  }, [load]);

  function openCreate() {
    setEditExisting(false);
    setEditKey("");
    setEditValue("");
    setEditOpen(true);
  }

  function openEdit(entry: KVEntry) {
    setEditExisting(true);
    setEditKey(entry.key);
    setEditValue(entry.value);
    setEditOpen(true);
  }

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await infra.putConsulKV(workspaceId, { key: editKey, value: editValue });
      setEditOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function onDelete(key: string) {
    if (!window.confirm(`确定删除键「${key}」？`)) return;
    setError(null);
    try {
      await infra.deleteConsulKV(workspaceId, key);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    }
  }

  return (
    <div className="stack-md">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="input max-w-xs"
          placeholder="键前缀过滤"
          value={prefix}
          onChange={(e) => setPrefix(e.target.value)}
        />
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          查询
        </button>
        <button type="button" className="btn ml-auto" onClick={openCreate}>
          新建键值
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无 KV 条目（Greenfield Mock，数据存 PostgreSQL）</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>键</th>
                <th>值</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.key}>
                  <td className="font-mono">{row.key}</td>
                  <td className="font-mono truncate max-w-md">{row.value}</td>
                  <td>{row.updated_at ? new Date(row.updated_at).toLocaleString() : "-"}</td>
                  <td>
                    <div className="flex gap-2">
                      <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => openEdit(row)}>
                        编辑
                      </button>
                      <button
                        type="button"
                        className="text-xs text-red-600 hover:underline"
                        onClick={() => void onDelete(row.key)}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={editOpen} onClose={() => setEditOpen(false)} title={editKey ? "编辑 KV" : "新建 KV"}>
        <form onSubmit={onSave} className="stack-md">
          <input
            className="input font-mono"
            placeholder="键，如 app/config/db"
            value={editKey}
            onChange={(e) => setEditKey(e.target.value)}
            required
            disabled={editExisting}
          />
          <textarea
            className="input min-h-32 font-mono"
            placeholder="值"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
          />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setEditOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={saving}>
              {saving ? "保存中…" : "保存"}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
