"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

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

function unboundRoles(member: WorkspaceMemberVM, roles: RoleVM[]) {
  const bound = new Set(member.roles.map((role) => role.id));
  return roles.filter((role) => !bound.has(role.id));
}

function isRoleBound(member: WorkspaceMemberVM, roleId: string) {
  return member.roles.some((role) => role.id === roleId);
}

export function MemberBindings({ workspaceId, members, roles, variant = "default" }: MemberBindingsProps) {
  const router = useRouter();
  const { confirm, ConfirmDialog } = useConfirmDialog();
  const [selectedRoles, setSelectedRoles] = useState<Record<string, string>>({});
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [successId, setSuccessId] = useState<string | null>(null);
  const [removingKey, setRemovingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const successTimerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (successTimerRef.current !== null) {
        window.clearTimeout(successTimerRef.current);
      }
    };
  }, []);

  function flashSuccess(memberId: string) {
    setSuccessId(memberId);
    if (successTimerRef.current !== null) {
      window.clearTimeout(successTimerRef.current);
    }
    successTimerRef.current = window.setTimeout(() => {
      setSuccessId((current) => (current === memberId ? null : current));
      successTimerRef.current = null;
    }, 2000);
  }

  async function onAssign(member: WorkspaceMemberVM) {
    const roleId = selectedRoles[member.member_id];
    if (!roleId) return;
    if (isRoleBound(member, roleId)) {
      setError("该成员已拥有此角色，无需重复绑定");
      setSelectedRoles((prev) => {
        const next = { ...prev };
        delete next[member.member_id];
        return next;
      });
      return;
    }
    setLoadingId(member.member_id);
    setError(null);
    try {
      await governance.assignRole(workspaceId, member.member_id, roleId);
      setSelectedRoles((prev) => {
        const next = { ...prev };
        delete next[member.member_id];
        return next;
      });
      flashSuccess(member.member_id);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "绑定失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onRemove(member: WorkspaceMemberVM, roleId: string, roleName: string) {
    const memberId = member.member_id;
    const key = `${memberId}:${roleId}`;
    const displayName = member.display_name || member.email;
    const ok = await confirm({ title: `确定解除 ${displayName} 的「${roleName}」角色？`, danger: true });
    if (!ok) return;
    setRemovingKey(key);
    setError(null);
    try {
      await governance.removeRoleBinding(workspaceId, memberId, roleId);
      router.refresh();
    } catch (err) {
      const message =
        err instanceof BffError
          ? err.code === "GOVERNANCE_BINDING_NOT_FOUND"
            ? "该角色绑定已不存在，请刷新页面"
            : err.message
          : "解绑失败";
      setError(message);
      router.refresh();
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
      <><div className="admin-members-list">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {members.map((member) => {
          const accent = memberAccent(member.member_id);
          const displayName = member.display_name || member.email.split("@")[0];
          const availableRoles = unboundRoles(member, roles);
          const selectedRoleId = selectedRoles[member.member_id] ?? "";
          const canAssign =
            Boolean(selectedRoleId) &&
            !isRoleBound(member, selectedRoleId) &&
            loadingId !== member.member_id &&
            successId !== member.member_id;
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
                        onClick={() => void onRemove(member, role.id, role.name)}
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
                  value={selectedRoleId}
                  aria-label={`为 ${displayName} 选择角色`}
                  disabled={availableRoles.length === 0 || loadingId === member.member_id}
                  onChange={(e) =>
                    setSelectedRoles((prev) => ({
                      ...prev,
                      [member.member_id]: e.target.value,
                    }))
                  }
                >
                  <option value="">
                    {availableRoles.length === 0 ? "角色已全部绑定" : "选择角色"}
                  </option>
                  {availableRoles.map((role) => (
                    <option key={role.id} value={role.id}>
                      {role.name}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  className={`admin-member-assign-btn ${canAssign ? "btn" : successId === member.member_id ? "btn admin-member-assign-btn-success" : "btn-secondary"}`}
                  disabled={!canAssign}
                  onClick={() => void onAssign(member)}
                >
                  {loadingId === member.member_id ? "绑定中…" : successId === member.member_id ? "已绑定" : "绑定"}
                </button>
              </div>
            </article>
          );
        })}
      </div><ConfirmDialog /></>
    );
  }

  return (
    <><div className="space-y-4">
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
            {members.map((member) => {
              const availableRoles = unboundRoles(member, roles);
              const selectedRoleId = selectedRoles[member.member_id] ?? "";
              const canAssign =
                Boolean(selectedRoleId) &&
                !isRoleBound(member, selectedRoleId) &&
                loadingId !== member.member_id &&
                successId !== member.member_id;
              return (
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
                            onClick={() => void onRemove(member, role.id, role.name)}
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
                      value={selectedRoleId}
                      disabled={availableRoles.length === 0 || loadingId === member.member_id}
                      onChange={(e) =>
                        setSelectedRoles((prev) => ({
                          ...prev,
                          [member.member_id]: e.target.value,
                        }))
                      }
                    >
                      <option value="">
                        {availableRoles.length === 0 ? "角色已全部绑定" : "选择角色"}
                      </option>
                      {availableRoles.map((role) => (
                        <option key={role.id} value={role.id}>
                          {role.name}
                        </option>
                      ))}
                    </select>
                    <button
                      className={`shrink-0 ${canAssign ? "btn" : successId === member.member_id ? "btn admin-member-assign-btn-success" : "btn-secondary"}`}
                      type="button"
                      disabled={!canAssign}
                      onClick={() => void onAssign(member)}
                    >
                      {loadingId === member.member_id ? "绑定中..." : successId === member.member_id ? "已绑定" : "绑定"}
                    </button>
                  </div>
                </td>
              </tr>
            );
            })}
          </tbody>
        </table>
      </div>
    </div><ConfirmDialog /></>
  );
}
