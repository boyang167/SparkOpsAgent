"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface MemberBindingsProps {
  workspaceId: string;
  members: WorkspaceMemberVM[];
  roles: RoleVM[];
  variant?: "default" | "admin";
}

function memberInitial(name: string, email: string) {
  const source = (name || email).trim();
  return source.charAt(0).toUpperCase() || "?";
}

function memberAccent(id: string) {
  const palette = ["#6366f1", "#0ea5e9", "#10b981", "#f59e0b", "#ec4899", "#8b5cf6"];
  let hash = 0;
  for (let i = 0; i < id.length; i += 1) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash);
  }
  return palette[Math.abs(hash) % palette.length];
}

const ROLE_CHIP_CLASS: Record<string, string> = {
  ADMIN: "admin-role-chip admin-role-chip-admin",
  DEVELOPER: "admin-role-chip admin-role-chip-dev",
  OPERATOR: "admin-role-chip admin-role-chip-ops",
  REVIEWER: "admin-role-chip admin-role-chip-review",
  VIEWER: "admin-role-chip admin-role-chip-view",
};

function roleChipClass(name: string) {
  const key = name.toUpperCase().replace(/\s+/g, "_");
  return ROLE_CHIP_CLASS[key] ?? "admin-role-chip";
}

export function MemberBindings({ workspaceId, members, roles, variant = "default" }: MemberBindingsProps) {
  const router = useRouter();
  const [selectedRoles, setSelectedRoles] = useState<Record<string, string>>({});
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [removingKey, setRemovingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onAssign(memberId: string) {
    const roleId = selectedRoles[memberId];
    if (!roleId) return;
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
    return variant === "admin" ? (
      <div className="admin-members-empty">暂无工作区成员，邀请第一位成员开始协作。</div>
    ) : (
      <div className="empty-state">暂无 Workspace 成员</div>
    );
  }

  if (variant === "admin") {
    return (
      <div className="admin-members-list">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {members.map((member) => {
          const accent = memberAccent(member.member_id);
          const displayName = member.display_name || member.email.split("@")[0];
          return (
            <article key={member.member_id} className="admin-member-row">
              <div className="admin-member-main">
                <span
                  className="admin-member-avatar"
                  style={{
                    background: `${accent}18`,
                    color: accent,
                    boxShadow: `inset 0 0 0 1px ${accent}30`,
                  }}
                  aria-hidden
                >
                  {memberInitial(member.display_name ?? "", member.email)}
                </span>
                <div className="admin-member-copy">
                  <p className="admin-member-name">{displayName}</p>
                  <p className="admin-member-email">{member.email}</p>
                </div>
              </div>

              <div className="admin-member-roles">
                {member.roles.length === 0 ? (
                  <span className="admin-member-unassigned">未分配角色</span>
                ) : (
                  member.roles.map((role) => (
                    <span key={role.id} className={roleChipClass(role.name)}>
                      {role.name}
                      <button
                        type="button"
                        className="admin-role-chip-remove"
                        title="解除绑定"
                        disabled={removingKey === `${member.member_id}:${role.id}`}
                        onClick={() => void onRemove(member.member_id, role.id)}
                      >
                        ×
                      </button>
                    </span>
                  ))
                )}
              </div>

              <div className="admin-member-assign">
                <select
                  className="admin-member-select"
                  value={selectedRoles[member.member_id] ?? ""}
                  aria-label={`为 ${displayName} 选择角色`}
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
                  type="button"
                  className="btn admin-member-assign-btn"
                  disabled={!selectedRoles[member.member_id] || loadingId === member.member_id}
                  onClick={() => void onAssign(member.member_id)}
                >
                  {loadingId === member.member_id ? "…" : "绑定"}
                </button>
              </div>
            </article>
          );
        })}
      </div>
    );
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
                            onClick={() => void onRemove(member.member_id, role.id)}
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
                      onClick={() => void onAssign(member.member_id)}
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
