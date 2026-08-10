"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { knowledge } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { DocumentVM } from "@sparkops/contracts/knowledge";

interface DocumentPanelProps {
  workspaceId: string;
  kbId: string;
  documents: DocumentVM[];
}

export function DocumentPanel({ workspaceId, kbId, documents }: DocumentPanelProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await knowledge.createDocument(kbId, workspaceId, { title, content });
      setOpen(false);
      setTitle("");
      setContent("");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  async function onDelete(doc: DocumentVM) {
    if (!window.confirm(`确定删除文档「${doc.title}」？`)) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await knowledge.archiveDocument(doc.id, workspaceId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="stack-md">
      <div className="flex items-center justify-between gap-4">
        <h2 className="section-title">文档</h2>
        {!open ? (
          <button className="btn" type="button" onClick={() => setOpen(true)}>
            新建文档
          </button>
        ) : null}
      </div>

      {open ? (
        <form className="form-panel space-y-4" onSubmit={onCreate}>
          <div>
            <label className="label" htmlFor="doc-title">
              标题
            </label>
            <input
              id="doc-title"
              className="input"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </div>
          <div>
            <label className="label" htmlFor="doc-content">
              内容
            </label>
            <textarea
              id="doc-content"
              className="input min-h-32 resize-y"
              value={content}
              onChange={(e) => setContent(e.target.value)}
            />
          </div>
          {error ? <p className="error">{error}</p> : null}
          <div className="flex gap-2">
            <button className="btn" type="submit" disabled={loading}>
              {loading ? "创建中..." : "创建"}
            </button>
            <button className="btn-secondary" type="button" onClick={() => setOpen(false)}>
              取消
            </button>
          </div>
        </form>
      ) : null}

      {documents.length === 0 && !open ? (
        <div className="empty-state">暂无文档，点击「新建文档」添加。</div>
      ) : (
        <div className="stack-md">
          {documents.map((doc) => (
            <DocumentRow
              key={doc.id}
              doc={doc}
              workspaceId={workspaceId}
              editing={editingId === doc.id}
              onEdit={() => setEditingId(doc.id)}
              onCancel={() => setEditingId(null)}
              onDelete={() => onDelete(doc)}
              onSaved={() => {
                setEditingId(null);
                router.refresh();
              }}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function DocumentRow({
  doc,
  workspaceId,
  editing,
  onEdit,
  onCancel,
  onDelete,
  onSaved,
}: {
  doc: DocumentVM;
  workspaceId: string;
  editing: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onDelete: () => void;
  onSaved: () => void;
}) {
  const [title, setTitle] = useState(doc.title);
  const [content, setContent] = useState(doc.content);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await knowledge.updateDocument(doc.id, workspaceId, { title, content });
      onSaved();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  if (editing) {
    return (
      <form className="form-panel space-y-3" onSubmit={onSave}>
        <input className="input" value={title} onChange={(e) => setTitle(e.target.value)} required />
        <textarea
          className="input min-h-32 resize-y"
          value={content}
          onChange={(e) => setContent(e.target.value)}
        />
        {error ? <p className="error">{error}</p> : null}
        <div className="flex gap-2">
          <button className="btn" type="submit" disabled={loading}>
            {loading ? "保存中..." : "保存"}
          </button>
          <button className="btn-secondary" type="button" onClick={onCancel}>
            取消
          </button>
        </div>
      </form>
    );
  }

  return (
    <div className="card">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="font-semibold">{doc.title}</p>
          <p className="text-muted mt-2 whitespace-pre-wrap text-sm line-clamp-6">
            {doc.content || "（空内容）"}
          </p>
        </div>
        <div className="flex shrink-0 flex-col gap-2">
          <button className="btn-secondary" type="button" onClick={onEdit}>
            编辑
          </button>
          <button
            className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
            type="button"
            onClick={onDelete}
          >
            删除
          </button>
        </div>
      </div>
    </div>
  );
}
