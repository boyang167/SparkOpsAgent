"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";

interface MemberBindingsProps {
  workspaceId: string;
  members: WorkspaceMemberVM[];
  roles: RoleVM[];
}

export function MemberBindings({ workspaceId, members, roles }: MemberBindingsProps) {
  const router = useRouter();
  const [selectedRoles, setSelectedRoles] = useState<Record<string, string>>({});
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [removingKey, setRemovingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onAssign(memberId: string) {
    const roleId = selectedRoles[memberId];
    if (!roleId) {
      return;
    }
    setLoadingId(memberId);
    setError(null);
    try {
      await governance.assignRole(workspaceId, memberId, roleId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "绑定失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onRemove(memberId: string, roleId: string) {
    const key = `${memberId}:${roleId}`;
    setRemovingKey(key);
    setError(null);
    try {
      await governance.removeRoleBinding(workspaceId, memberId, roleId);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "解绑失败");
    } finally {
      setRemovingKey(null);
    }
  }

  if (members.length === 0) {
    return <div className="empty-state">暂无 Workspace 成员</div>;
  }

  return (
    <div className="space-y-4">
      {error ? <p className="error">{error}</p> : null}
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>成员</th>
              <th>已绑定角色</th>
              <th>分配角色</th>
            </tr>
          </thead>
          <tbody>
            {members.map((member) => (
              <tr key={member.member_id}>
                <td>
                  <p className="font-medium">{member.display_name || member.email}</p>
                  <p className="text-xs text-muted mt-0.5">{member.email}</p>
                </td>
                <td>
                  {member.roles.length === 0 ? (
                    <span className="text-[var(--muted)]">未分配</span>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {member.roles.map((role) => (
                        <span
                          key={role.id}
                          className="badge badge-neutral inline-flex items-center gap-1 !pl-2.5 !pr-1"
                        >
                          {role.name}
                          <button
                            type="button"
                            className="rounded-full px-1 text-muted hover:text-red-600"
                            title="解除绑定"
                            disabled={removingKey === `${member.member_id}:${role.id}`}
                            onClick={() => onRemove(member.member_id, role.id)}
                          >
                            ×
                          </button>
                        </span>
                      ))}
                    </div>
                  )}
                </td>
                <td>
                  <div className="flex flex-col sm:flex-row gap-2 min-w-[220px]">
                    <select
                      className="input"
                      value={selectedRoles[member.member_id] ?? ""}
                      onChange={(e) =>
                        setSelectedRoles((prev) => ({
                          ...prev,
                          [member.member_id]: e.target.value,
                        }))
                      }
                    >
                      <option value="">选择角色</option>
                      {roles.map((role) => (
                        <option key={role.id} value={role.id}>
                          {role.name}
                        </option>
                      ))}
                    </select>
                    <button
                      className="btn shrink-0"
                      type="button"
                      disabled={!selectedRoles[member.member_id] || loadingId === member.member_id}
                      onClick={() => onAssign(member.member_id)}
                    >
                      {loadingId === member.member_id ? "绑定中..." : "绑定"}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
