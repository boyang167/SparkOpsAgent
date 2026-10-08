"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import type { RoleVM } from "@sparkops/contracts/governance";
import { isProtectedRole, roleDisplayName } from "./roles/role-utils";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

function permissionLabel(value: string) {
  return PERMISSION_OPTIONS.find((p) => p.value === value)?.label ?? value;
}

interface RoleListProps {
  workspaceId: string;
  roles: RoleVM[];
}

export function RoleList({ workspaceId, roles }: RoleListProps) {
  const router = useRouter();
  const { confirm, ConfirmDialog } = useConfirmDialog();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  function startEdit(role: RoleVM) {
    setEditingId(role.id);
    setPermissions([...role.permissions]);
    setError(null);
  }

  function togglePermission(value: string) {
    setPermissions((prev) =>
      prev.includes(value) ? prev.filter((p) => p !== value) : [...prev, value],
    );
  }

  async function onSave(roleId: string) {
    if (permissions.length === 0) {
      setError("至少选择一个权限");
      return;
    }
    setLoadingId(roleId);
    setError(null);
    try {
      await governance.updateRole(workspaceId, roleId, { permissions });
      setEditingId(null);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onDelete(role: RoleVM) {
    if (isProtectedRole(role.name)) {
      return;
    }
    const ok = await confirm({ title: `确定删除角色「${role.name}」？相关成员绑定也会一并移除。`, danger: true });
    if (!ok) return;
    setLoadingId(role.id);
    setError(null);
    try {
      await governance.deleteRole(workspaceId, role.id);
      if (editingId === role.id) {
        setEditingId(null);
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoadingId(null);
    }
  }

  if (roles.length === 0) {
    return <div className="empty-state">暂无角色</div>;
  }

  return (
    <><div className="space-y-4">
      {error ? <p className="error">{error}</p> : null}
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>权限</th>
              <th style={{ width: "9rem" }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {roles.map((role) => {
              const isEditing = editingId === role.id;
              const isProtected = isProtectedRole(role.name);
              return (
                <tr key={role.id}>
                  <td className="font-medium">
                    {roleDisplayName(role.name)}
                    {isProtected ? (
                      <span className="badge badge-neutral ml-2">系统</span>
                    ) : null}
                  </td>
                  <td>
                    {isEditing ? (
                      <div className="grid gap-2 sm:grid-cols-2">
                        {PERMISSION_OPTIONS.map((option) => (
                          <label key={option.value} className="flex items-center gap-2 text-sm">
                            <input
                              type="checkbox"
                              checked={permissions.includes(option.value)}
                              onChange={() => togglePermission(option.value)}
                            />
                            {option.label}
                          </label>
                        ))}
                      </div>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {role.permissions.map((perm) => (
                          <span key={perm} className="badge badge-primary">
                            {permissionLabel(perm)}
                          </span>
                        ))}
                      </div>
                    )}
                  </td>
                  <td>
                    <div className="flex flex-col gap-2">
                      {isEditing ? (
                        <>
                          <button
                            className="btn text-xs py-2"
                            type="button"
                            disabled={loadingId === role.id}
                            onClick={() => onSave(role.id)}
                          >
                            {loadingId === role.id ? "保存中..." : "保存"}
                          </button>
                          <button
                            className="btn-secondary text-xs py-2"
                            type="button"
                            onClick={() => setEditingId(null)}
                          >
                            取消
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            className="btn-secondary text-xs py-2"
                            type="button"
                            disabled={loadingId === role.id}
                            onClick={() => startEdit(role)}
                          >
                            编辑权限
                          </button>
                          {!isProtected ? (
                            <button
                              className="text-xs py-2 text-red-600 hover:underline"
                              type="button"
                              disabled={loadingId === role.id}
                              onClick={() => onDelete(role)}
                            >
                              删除
                            </button>
                          ) : null}
                        </>
                      )}
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
