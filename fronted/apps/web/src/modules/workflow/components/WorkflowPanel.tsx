"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { workflow } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface CreateWorkflowFormProps {
  workspaceId: string;
  projectId: string;
}

export function CreateWorkflowForm({ workspaceId, projectId }: CreateWorkflowFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [definition, setDefinition] = useState("{}");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await workflow.createWorkflow(workspaceId, projectId, {
        name,
        description,
        definition,
      });
      setOpen(false);
      setName("");
      setDescription("");
      setDefinition("{}");
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
        新建 Workflow
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">新建 Workflow</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="workflow-name">
          名称
        </label>
        <input
          id="workflow-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>
      <div>
        <label className="label" htmlFor="workflow-desc">
          描述
        </label>
        <textarea
          id="workflow-desc"
          className="input min-h-20 resize-y"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>
      <div>
        <label className="label" htmlFor="workflow-def">
          定义 (JSON)
        </label>
        <textarea
          id="workflow-def"
          className="input min-h-24 resize-y font-mono text-sm"
          value={definition}
          onChange={(e) => setDefinition(e.target.value)}
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading}>
        {loading ? "创建中..." : "创建"}
      </button>
    </form>
  );
}

interface WorkflowSettingsProps {
  workspaceId: string;
  workflows: WorkflowVM[];
}

export function WorkflowSettings({ workspaceId, workflows }: WorkflowSettingsProps) {
  const router = useRouter();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { confirm, ConfirmDialog } = useConfirmDialog();

  async function onDelete(wf: WorkflowVM) {
    const ok = await confirm({ title: `确定删除 Workflow「${wf.name}」？`, danger: true });
    if (!ok) return;
    setLoadingId(wf.id);
    setError(null);
    try {
      await workflow.archiveWorkflow(wf.id, workspaceId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoadingId(null);
    }
  }

  if (workflows.length === 0) {
    return null;
  }

  return (
    <>
    <div className="stack-md mt-4">
      {workflows.map((wf) => (
        <WorkflowEditRow
          key={wf.id}
          workspaceId={workspaceId}
          workflow={wf}
          editing={editingId === wf.id}
          loading={loadingId === wf.id}
          onEdit={() => setEditingId(wf.id)}
          onCancel={() => setEditingId(null)}
          onDelete={() => onDelete(wf)}
          onSaved={() => {
            setEditingId(null);
            router.refresh();
          }}
        />
      ))}
      {error ? <p className="error">{error}</p> : null}
    </div>
    <ConfirmDialog />
    </>
  );
}

function WorkflowEditRow({
  workspaceId,
  workflow: wf,
  editing,
  loading,
  onEdit,
  onCancel,
  onDelete,
  onSaved,
}: {
  workspaceId: string;
  workflow: WorkflowVM;
  editing: boolean;
  loading: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onDelete: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(wf.name);
  const [description, setDescription] = useState(wf.description);
  const [definition, setDefinition] = useState(wf.definition ?? "{}");
  const [saveLoading, setSaveLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setSaveLoading(true);
    setError(null);
    try {
      await workflow.updateWorkflow(wf.id, workspaceId, { name, description, definition });
      onSaved();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaveLoading(false);
    }
  }

  if (!editing) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-[var(--border)] px-4 py-3">
        <span className="font-medium">{wf.name}</span>
        <div className="flex gap-2">
          <button className="btn-secondary" type="button" onClick={onEdit}>
            编辑
          </button>
          <button
            className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
            type="button"
            onClick={onDelete}
            disabled={loading}
          >
            {loading ? "删除中..." : "删除"}
          </button>
        </div>
      </div>
    );
  }

  return (
    <form className="form-panel space-y-3" onSubmit={onSave}>
      <input className="input" value={name} onChange={(e) => setName(e.target.value)} required />
      <textarea
        className="input min-h-16 resize-y"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
      />
      <textarea
        className="input min-h-20 resize-y font-mono text-sm"
        value={definition}
        onChange={(e) => setDefinition(e.target.value)}
      />
      {error ? <p className="error">{error}</p> : null}
      <div className="flex gap-2">
        <button className="btn" type="submit" disabled={saveLoading}>
          {saveLoading ? "保存中..." : "保存"}
        </button>
        <button className="btn-secondary" type="button" onClick={onCancel}>
          取消
        </button>
      </div>
    </form>
  );
}
