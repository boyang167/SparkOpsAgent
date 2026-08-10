"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

export function ProclaimsPanel() {
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listProclaims());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    try {
      await platform.createProclaim({ title, body, proclaim_type: "notice", proclaim_receive: "all" });
      setOpen(false);
      setTitle("");
      setBody("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "发布失败");
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn" onClick={() => setOpen(true)}>发布公告</button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无公告</div>
      ) : (
        <div className="stack-sm">
          {items.map((row) => (
            <article key={String(row.id)} className="rounded-lg border border-[var(--border)] p-4">
              <h3 className="font-medium">{String(row.title ?? "-")}</h3>
              <p className="text-sm text-muted mt-2 whitespace-pre-wrap">{String(row.body ?? row.content ?? "")}</p>
            </article>
          ))}
        </div>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="发布公告">
        <form onSubmit={onCreate} className="stack-md">
          <input className="input" placeholder="标题" value={title} onChange={(e) => setTitle(e.target.value)} required />
          <textarea className="input min-h-24" placeholder="内容" value={body} onChange={(e) => setBody(e.target.value)} />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>取消</button>
            <button type="submit" className="btn">发布</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
