"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { agent } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface AgentSettingsPanelProps {
  workspaceId: string;
  agentId: string;
  projectId: string;
  agentName: string;
}

export function AgentSettingsPanel({
  workspaceId,
  agentId,
  projectId,
  agentName,
}: AgentSettingsPanelProps) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(agentName);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { confirm, ConfirmDialog } = useConfirmDialog();

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await agent.updateAgent(agentId, workspaceId, { name });
      setEditing(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoading(false);
    }
  }

  async function onDelete() {
    const ok = await confirm({
      title: `确定删除 Agent「${agentName}」？`,
      description: "归档后不再出现在列表中。",
      danger: true,
    });
    if (!ok) return;
    setLoading(true);
    setError(null);
    try {
      await agent.archiveAgent(agentId, workspaceId);
      router.push(`/projects/${projectId}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">Agent 设置</h2>
          <p className="text-muted mt-1">修改名称或归档删除</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {!editing ? (
            <button className="btn-secondary" type="button" onClick={() => setEditing(true)}>
              编辑名称
            </button>
          ) : null}
          <button
            className="btn-secondary !text-red-600 !border-red-200 hover:!bg-red-50"
            type="button"
            onClick={onDelete}
            disabled={loading}
          >
            {loading && !editing ? "删除中..." : "删除 Agent"}
          </button>
        </div>
      </div>

      {editing ? (
        <form className="form-panel space-y-4" onSubmit={onSave}>
          <div>
            <label className="label" htmlFor="edit-agent-name">
              名称
            </label>
            <input
              id="edit-agent-name"
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          {error ? <p className="error">{error}</p> : null}
          <div className="flex gap-2">
            <button className="btn" type="submit" disabled={loading}>
              {loading ? "保存中..." : "保存"}
            </button>
            <button
              className="btn-secondary"
              type="button"
              onClick={() => {
                setEditing(false);
                setName(agentName);
                setError(null);
              }}
            >
              取消
            </button>
          </div>
        </form>
      ) : null}
    </section>
    <ConfirmDialog />
    </>
  );
}
