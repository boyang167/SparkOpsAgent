"use client";

import { useCallback, useEffect, useState } from "react";
import { workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ProjectMemberVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";

const ROLE_LABELS: Record<string, string> = {
  lead: "负责人",
  member: "成员",
  viewer: "只读",
};

interface ProjectMembersPanelProps {
  workspaceId: string;
  projectId: string;
}

export function ProjectMembersPanel({ workspaceId, projectId }: ProjectMembersPanelProps) {
  const [members, setMembers] = useState<ProjectMemberVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [memberId, setMemberId] = useState("");
  const [role, setRole] = useState<"lead" | "member" | "viewer">("member");
  const [adding, setAdding] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setMembers(await workspace.listProjectMembers(workspaceId, projectId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载成员失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projectId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!memberId.trim()) return;
    setAdding(true);
    setError(null);
    try {
      await workspace.addProjectMember(workspaceId, projectId, { member_id: memberId.trim(), role });
      setMemberId("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "添加失败");
    } finally {
      setAdding(false);
    }
  }

  async function onChangeRole(id: string, nextRole: "lead" | "member" | "viewer") {
    setActionId(id);
    try {
      await workspace.updateProjectMemberRole(workspaceId, projectId, id, nextRole);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新失败");
    } finally {
      setActionId(null);
    }
  }

  async function onRemove(id: string) {
    setActionId(`remove:${id}`);
    try {
      await workspace.removeProjectMember(workspaceId, projectId, id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "移除失败");
    } finally {
      setActionId(null);
    }
  }

  return (
    <section className="section">
      <div className="section-header">
        <div>
          <h2 className="section-title">项目成员</h2>
          <p className="text-muted mt-1">项目级 ACL：lead / member / viewer</p>
        </div>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <form onSubmit={onAdd} className="flex flex-col sm:flex-row gap-2 mb-4">
        <input
          className="input flex-1"
          placeholder="成员 UUID"
          value={memberId}
          onChange={(e) => setMemberId(e.target.value)}
          required
        />
        <select className="input sm:w-32" value={role} onChange={(e) => setRole(e.target.value as typeof role)}>
          <option value="member">成员</option>
          <option value="lead">负责人</option>
          <option value="viewer">只读</option>
        </select>
        <button type="submit" className="btn shrink-0" disabled={adding}>
          {adding ? "添加中…" : "添加成员"}
        </button>
      </form>
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : members.length === 0 ? (
        <div className="empty-state">暂无项目成员</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>成员</th>
                <th>角色</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.member_id}>
                  <td>
                    <p className="font-medium">{m.display_name || m.email}</p>
                    <p className="text-xs text-muted">{m.email}</p>
                  </td>
                  <td>
                    <span className="badge badge-neutral">{ROLE_LABELS[m.role] ?? m.role}</span>
                  </td>
                  <td>
                    <div className="flex flex-wrap gap-2">
                      {m.role !== "lead" ? (
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          disabled={actionId === m.member_id}
                          onClick={() => onChangeRole(m.member_id, "lead")}
                        >
                          设为负责人
                        </button>
                      ) : null}
                      {m.role !== "member" ? (
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          disabled={actionId === m.member_id}
                          onClick={() => onChangeRole(m.member_id, "member")}
                        >
                          设为成员
                        </button>
                      ) : null}
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs text-red-600"
                        disabled={actionId === `remove:${m.member_id}`}
                        onClick={() => onRemove(m.member_id)}
                      >
                        移除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
