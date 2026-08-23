"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import type { RoleTreeVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MetricCard } from "@/components/ui/MetricCard";
import { RoleAuthorityEditor, type RoleAuthorityMode } from "./roles/RoleAuthorityEditor";
import {
  computeRoleStats,
  flattenRoleTree,
  isProtectedRole,
  parentRoleLabel,
  roleDisplayName,
} from "./roles/role-utils";

function permissionLabel(value: string) {
  return PERMISSION_OPTIONS.find((p) => p.value === value)?.label ?? value;
}

interface RoleTreePanelProps {
  workspaceId: string;
}

export function RoleTreePanel({ workspaceId }: RoleTreePanelProps) {
  const router = useRouter();
  const [tree, setTree] = useState<RoleTreeVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [loadingId, setLoadingId] = useState<string | null>(null);

  const [editorOpen, setEditorOpen] = useState(false);
  const [editorMode, setEditorMode] = useState<RoleAuthorityMode>("create");
  const [editorRole, setEditorRole] = useState<RoleTreeVM | null>(null);
  const [editorParentId, setEditorParentId] = useState<string | undefined>();

  const stats = useMemo(() => computeRoleStats(tree), [tree]);
  const rows = useMemo(() => flattenRoleTree(tree), [tree]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setTree(await governance.listRoleTree(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3200);
  }

  function openEditor(mode: RoleAuthorityMode, role?: RoleTreeVM | null, parentId?: string) {
    setEditorMode(mode);
    setEditorRole(role ?? null);
    setEditorParentId(parentId);
    setEditorOpen(true);
  }

  function openCreate(parentId?: string) {
    openEditor("create", null, parentId);
  }

  function openEdit(role: RoleTreeVM) {
    openEditor("edit", role);
  }

  function openCopy(role: RoleTreeVM) {
    openEditor("copy", role);
  }

  function openAuthority(role: RoleTreeVM) {
    openEditor("authority", role);
  }

  async function onDelete(role: RoleTreeVM) {
    if (!window.confirm(`确定删除角色「${role.name}」？`)) return;
    setLoadingId(role.id);
    setError(null);
    try {
      await governance.deleteRole(workspaceId, role.id);
      flashSuccess(`已删除角色「${role.name}」`);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoadingId(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="角色总数"
          value={stats.total}
          hint="含所有层级"
          tone="sky"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
          }
        />
        <MetricCard
          label="根角色"
          value={stats.rootCount}
          hint="无父级的角色"
          tone="violet"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M12 3v18M3 12h18" />
            </svg>
          }
        />
        <MetricCard
          label="最大层级"
          value={stats.maxDepth + 1}
          hint="树深度（含根）"
          tone="emerald"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M6 9H4.5a2.5 2.5 0 0 1 0-5H6M18 9h1.5a2.5 2.5 0 0 0 0-5H18M6 9v6M18 9v6M6 15H4.5a2.5 2.5 0 0 0 0 5H6M18 15h1.5a2.5 2.5 0 0 1 0 5H18" />
            </svg>
          }
        />
        <MetricCard
          label="系统角色"
          value={stats.systemCount}
          hint="如 admin"
          tone="amber"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
              <path d="M12 2 4 5v6c0 5.55 3.84 10.74 8 12 4.16-1.26 8-6.45 8-12V5l-8-3Z" />
            </svg>
          }
        />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted">管理角色层级与权限分配，参考 GVA 角色管理交互。</p>
        <button type="button" className="btn" onClick={() => openCreate()}>
          新增角色
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : rows.length === 0 ? (
        <div className="empty-state">暂无角色</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>角色 ID</th>
                <th>角色名称</th>
                <th>父级角色</th>
                <th>功能权限</th>
                <th className="min-w-[18rem]">操作</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((role) => (
                <tr key={role.id}>
                  <td className="font-mono text-xs text-muted">{role.id.slice(0, 8)}…</td>
                  <td className="font-medium" style={{ paddingLeft: `${role.depth * 1.25 + 0.75}rem` }}>
                    {role.depth > 0 ? "↳ " : null}
                    {roleDisplayName(role.name)}
                    {isProtectedRole(role.name) ? <span className="badge badge-neutral ml-2">系统</span> : null}
                  </td>
                  <td className="text-sm">{parentRoleLabel(tree, role.parent_id)}</td>
                  <td>
                    <div className="flex flex-wrap gap-1 max-w-xs">
                      {role.permissions.slice(0, 3).map((perm) => (
                        <span key={perm} className="badge badge-primary text-xs">
                          {permissionLabel(perm)}
                        </span>
                      ))}
                      {role.permissions.length > 3 ? (
                        <span className="badge badge-neutral text-xs">+{role.permissions.length - 3}</span>
                      ) : null}
                    </div>
                  </td>
                  <td>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        onClick={() => openAuthority(role)}
                      >
                        设置权限
                      </button>
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        onClick={() => openCreate(role.id)}
                      >
                        新增子角色
                      </button>
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        disabled={loadingId === role.id}
                        onClick={() => openCopy(role)}
                      >
                        拷贝
                      </button>
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        onClick={() => openEdit(role)}
                      >
                        编辑
                      </button>
                      {!isProtectedRole(role.name) ? (
                        <button
                          type="button"
                          className="text-xs text-red-600 hover:underline"
                          disabled={loadingId === role.id}
                          onClick={() => void onDelete(role)}
                        >
                          删除
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <RoleAuthorityEditor
        open={editorOpen}
        mode={editorMode}
        workspaceId={workspaceId}
        tree={tree}
        role={editorRole}
        parentId={editorParentId}
        onClose={() => setEditorOpen(false)}
        onSaved={() => {
          flashSuccess(
            editorMode === "create"
              ? "角色已创建"
              : editorMode === "copy"
                ? "角色已拷贝"
                : "角色权限已保存",
          );
          void load();
          router.refresh();
        }}
      />
    </div>
  );
}
