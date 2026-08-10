"use client";

import { Fragment, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import type { RoleTreeVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { RoleApiPolicyModal } from "./RoleApiPolicyModal";

function permissionLabel(value: string) {
  return PERMISSION_OPTIONS.find((p) => p.value === value)?.label ?? value;
}

interface RoleTreePanelProps {
  workspaceId: string;
}

function RoleTreeRows({
  nodes,
  depth,
  onCopy,
  onAddChild,
  onEdit,
  onDelete,
  onBindApi,
  loadingId,
}: {
  nodes: RoleTreeVM[];
  depth: number;
  onCopy: (role: RoleTreeVM) => void;
  onAddChild: (role: RoleTreeVM) => void;
  onEdit: (role: RoleTreeVM) => void;
  onDelete: (role: RoleTreeVM) => void;
  onBindApi: (role: RoleTreeVM) => void;
  loadingId: string | null;
}) {
  return (
    <>
      {nodes.map((role) => (
        <Fragment key={role.id}>
          <tr>
            <td className="font-medium" style={{ paddingLeft: `${depth * 1.25 + 0.75}rem` }}>
              {depth > 0 ? "↳ " : null}
              {role.name}
              {role.name === "admin" ? <span className="badge badge-neutral ml-2">系统</span> : null}
            </td>
            <td>
              <div className="flex flex-wrap gap-2">
                {role.permissions.map((perm) => (
                  <span key={perm} className="badge badge-primary">
                    {permissionLabel(perm)}
                  </span>
                ))}
              </div>
            </td>
            <td>
              <div className="flex flex-wrap gap-2">
                <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => onEdit(role)}>
                  设置权限
                </button>
                <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => onBindApi(role)}>
                  绑定 API
                </button>
                <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => onAddChild(role)}>
                  新增子角色
                </button>
                <button
                  type="button"
                  className="btn-secondary !py-1 !px-2 text-xs"
                  disabled={loadingId === role.id}
                  onClick={() => onCopy(role)}
                >
                  拷贝
                </button>
                {role.name !== "admin" ? (
                  <button
                    type="button"
                    className="text-xs text-red-600 hover:underline"
                    disabled={loadingId === role.id}
                    onClick={() => onDelete(role)}
                  >
                    删除
                  </button>
                ) : null}
              </div>
            </td>
          </tr>
          {role.children && role.children.length > 0 ? (
            <RoleTreeRows
              nodes={role.children}
              depth={depth + 1}
              onCopy={onCopy}
              onAddChild={onAddChild}
              onEdit={onEdit}
              onDelete={onDelete}
              onBindApi={onBindApi}
              loadingId={loadingId}
            />
          ) : null}
        </Fragment>
      ))}
    </>
  );
}

export function RoleTreePanel({ workspaceId }: RoleTreePanelProps) {
  const router = useRouter();
  const [tree, setTree] = useState<RoleTreeVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [editRole, setEditRole] = useState<RoleTreeVM | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [createParentId, setCreateParentId] = useState<string | undefined>();
  const [createName, setCreateName] = useState("");
  const [createPermissions, setCreatePermissions] = useState<string[]>([]);
  const [bindRole, setBindRole] = useState<RoleTreeVM | null>(null);

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

  async function onCopy(role: RoleTreeVM) {
    setLoadingId(role.id);
    try {
      await governance.copyRole(workspaceId, role.id);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "拷贝失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onDelete(role: RoleTreeVM) {
    if (!window.confirm(`确定删除角色「${role.name}」？`)) return;
    setLoadingId(role.id);
    try {
      await governance.deleteRole(workspaceId, role.id);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onSavePermissions(e: React.FormEvent) {
    e.preventDefault();
    if (!editRole) return;
    setLoadingId(editRole.id);
    try {
      await governance.updateRole(workspaceId, editRole.id, permissions);
      setEditRole(null);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setLoadingId(null);
    }
  }

  async function onCreateRole(e: React.FormEvent) {
    e.preventDefault();
    setLoadingId("create");
    try {
      await governance.createRole(workspaceId, {
        name: createName,
        permissions: createPermissions,
        parent_id: createParentId,
      });
      setCreateOpen(false);
      setCreateName("");
      setCreatePermissions([]);
      setCreateParentId(undefined);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoadingId(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button
          type="button"
          className="btn"
          onClick={() => {
            setCreateParentId(undefined);
            setCreateOpen(true);
          }}
        >
          新增角色
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : tree.length === 0 ? (
        <div className="empty-state">暂无角色</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>角色名称</th>
                <th>权限</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <RoleTreeRows
                nodes={tree}
                depth={0}
                onCopy={onCopy}
                onAddChild={(role) => {
                  setCreateParentId(role.id);
                  setCreateOpen(true);
                }}
                onEdit={(role) => {
                  setEditRole(role);
                  setPermissions([...role.permissions]);
                }}
                onDelete={onDelete}
                onBindApi={(role) => setBindRole(role)}
                loadingId={loadingId}
              />
            </tbody>
          </table>
        </div>
      )}

      <Modal open={Boolean(editRole)} onClose={() => setEditRole(null)} title={editRole ? `设置权限 · ${editRole.name}` : ""}>
        <form onSubmit={onSavePermissions} className="stack-md">
          <div className="grid gap-2 sm:grid-cols-2">
            {PERMISSION_OPTIONS.map((option) => (
              <label key={option.value} className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={permissions.includes(option.value)}
                  onChange={() =>
                    setPermissions((prev) =>
                      prev.includes(option.value)
                        ? prev.filter((p) => p !== option.value)
                        : [...prev, option.value],
                    )
                  }
                />
                {option.label}
              </label>
            ))}
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setEditRole(null)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={permissions.length === 0}>
              保存
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title={createParentId ? "新增子角色" : "新增角色"}
      >
        <form onSubmit={onCreateRole} className="stack-md">
          <input
            className="input"
            placeholder="角色名称"
            value={createName}
            onChange={(e) => setCreateName(e.target.value)}
            required
          />
          <div className="grid gap-2 sm:grid-cols-2">
            {PERMISSION_OPTIONS.map((option) => (
              <label key={option.value} className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={createPermissions.includes(option.value)}
                  onChange={() =>
                    setCreatePermissions((prev) =>
                      prev.includes(option.value)
                        ? prev.filter((p) => p !== option.value)
                        : [...prev, option.value],
                    )
                  }
                />
                {option.label}
              </label>
            ))}
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={createPermissions.length === 0}>
              创建
            </button>
          </div>
        </form>
      </Modal>

      <RoleApiPolicyModal
        workspaceId={workspaceId}
        role={bindRole}
        onClose={() => setBindRole(null)}
        onSaved={() => void load()}
      />
    </div>
  );
}
