"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { knowledge } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { KnowledgeBaseVM } from "@sparkops/contracts/knowledge";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface CreateKnowledgeBaseFormProps {
  workspaceId: string;
}

export function CreateKnowledgeBaseForm({ workspaceId }: CreateKnowledgeBaseFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const kb = await knowledge.createKnowledgeBase(workspaceId, { name, description });
      setOpen(false);
      setName("");
      setDescription("");
      router.push(`/knowledge/${kb.id}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  if (!open) {
    return (
      <button className="btn" type="button" onClick={() => setOpen(true)}>
        新建知识库
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">新建知识库</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="kb-name">
          名称
        </label>
        <input
          id="kb-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="kb-desc">
          描述
        </label>
        <textarea
          id="kb-desc"
          className="input min-h-20 resize-y"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading}>
        {loading ? "创建中..." : "创建"}
      </button>
    </form>
  );
}

interface KnowledgeBaseSettingsProps {
  workspaceId: string;
  kb: KnowledgeBaseVM;
}

export function KnowledgeBaseSettings({ workspaceId, kb }: KnowledgeBaseSettingsProps) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(kb.name);
  const [description, setDescription] = useState(kb.description);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { confirm, ConfirmDialog } = useConfirmDialog();

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await knowledge.updateKnowledgeBase(kb.id, workspaceId, { name, description });
      setEditing(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  async function onDelete() {
    const ok = await confirm({ title: `确定删除知识库「${kb.name}」？`, danger: true });
    if (!ok) return;
    setLoading(true);
    setError(null);
    try {
      await knowledge.archiveKnowledgeBase(kb.id, workspaceId);
      router.push("/knowledge");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoading(false);
    }
  }

  if (!editing) {
    return (
      <>
      <div className="flex flex-wrap items-center gap-2">
        <button className="btn-secondary" type="button" onClick={() => setEditing(true)}>
          编辑知识库
        </button>
        <button
          className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
          type="button"
          onClick={onDelete}
          disabled={loading}
        >
          {loading ? "删除中..." : "删除知识库"}
        </button>
        {error ? <p className="error">{error}</p> : null}
      </div>
      <ConfirmDialog />
      </>
    );
  }

  return (
    <>
    <form className="form-panel space-y-4" onSubmit={onSave}>
      <div>
        <label className="label" htmlFor="edit-kb-name">
          名称
        </label>
        <input
          id="edit-kb-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="edit-kb-desc">
          描述
        </label>
        <textarea
          id="edit-kb-desc"
          className="input min-h-20 resize-y"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <div className="flex gap-2">
        <button className="btn" type="submit" disabled={loading}>
          {loading ? "保存中..." : "保存"}
        </button>
        <button className="btn-secondary" type="button" onClick={() => setEditing(false)}>
          取消
        </button>
      </div>
    </form>
    <ConfirmDialog />
    </>
  );
}
