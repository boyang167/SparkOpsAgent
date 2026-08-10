"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { workflow } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkflowVM } from "@sparkops/contracts/workflow";

interface WorkflowDetailPanelProps {
  workspaceId: string;
  projectId: string;
  workflow: WorkflowVM;
}

export function WorkflowDetailPanel({ workspaceId, projectId, workflow: wf }: WorkflowDetailPanelProps) {
  const router = useRouter();
  const [name, setName] = useState(wf.name);
  const [description, setDescription] = useState(wf.description);
  const [definition, setDefinition] = useState(wf.definition ?? "{}");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await workflow.updateWorkflow(wf.id, workspaceId, { name, description, definition });
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  async function onActivate() {
    setLoading(true);
    setError(null);
    try {
      await workflow.activateWorkflow(wf.id, workspaceId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "激活失败");
    } finally {
      setLoading(false);
    }
  }

  async function onDelete() {
    if (!window.confirm(`确定删除 Workflow「${wf.name}」？`)) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await workflow.archiveWorkflow(wf.id, workspaceId);
      router.push(`/projects/${projectId}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="stack-lg">
      <div className="flex flex-wrap items-center gap-2">
        <span className={`badge ${wf.status === "active" ? "badge-success" : "badge-neutral"}`}>
          {wf.status}
        </span>
        {wf.status === "draft" ? (
          <button className="btn-secondary" type="button" onClick={onActivate} disabled={loading}>
            激活
          </button>
        ) : null}
        <button
          className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
          type="button"
          onClick={onDelete}
          disabled={loading}
        >
          删除
        </button>
      </div>

      <form className="section stack-md" onSubmit={onSave}>
        <div>
          <label className="label" htmlFor="wf-name">
            名称
          </label>
          <input
            id="wf-name"
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label" htmlFor="wf-desc">
            描述
          </label>
          <textarea
            id="wf-desc"
            className="input min-h-20 resize-y"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
        <div>
          <label className="label" htmlFor="wf-def">
            定义 (JSON)
          </label>
          <textarea
            id="wf-def"
            className="input min-h-48 resize-y font-mono text-sm"
            value={definition}
            onChange={(e) => setDefinition(e.target.value)}
          />
        </div>
        {error ? <p className="error">{error}</p> : null}
        <button className="btn" type="submit" disabled={loading}>
          {loading ? "保存中..." : "保存"}
        </button>
      </form>
    </div>
  );
}
