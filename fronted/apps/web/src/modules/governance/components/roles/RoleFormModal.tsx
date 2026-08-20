"use client";

import { useEffect, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import type { RoleTreeVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { collectRoleParentOptions, isProtectedRole } from "./role-utils";

export type RoleFormMode = "create" | "edit" | "copy";

interface RoleFormModalProps {
  open: boolean;
  mode: RoleFormMode;
  tree: RoleTreeVM[];
  parentId?: string;
  role?: RoleTreeVM | null;
  workspaceId: string;
  onClose: () => void;
  onSaved: () => void;
}

export function RoleFormModal({
  open,
  mode,
  tree,
  parentId,
  role,
  workspaceId,
  onClose,
  onSaved,
}: RoleFormModalProps) {
  const [name, setName] = useState("");
  const [selectedParentId, setSelectedParentId] = useState<string>("");
  const [permissions, setPermissions] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    if (mode === "edit" && role) {
      setName(role.name);
      setSelectedParentId(role.parent_id ?? "");
      setPermissions([...role.permissions]);
      return;
    }
    if (mode === "copy" && role) {
      setName(`${role.name}_copy`);
      setSelectedParentId(role.parent_id ?? "");
      setPermissions([...role.permissions]);
      return;
    }
    setName("");
    setSelectedParentId(parentId ?? "");
    setPermissions([]);
  }, [open, mode, role, parentId]);

  const parentOptions = collectRoleParentOptions(tree, mode === "edit" ? role?.id : undefined);
  const title =
    mode === "create"
      ? parentId
        ? "新增子角色"
        : "新增角色"
      : mode === "edit"
        ? "编辑角色"
        : "拷贝角色";

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      if (mode === "create") {
        if (permissions.length === 0) {
          setError("至少选择一个功能权限");
          return;
        }
        await governance.createRole(workspaceId, {
          name: name.trim(),
          permissions,
          parent_id: selectedParentId || undefined,
        });
      } else if (mode === "edit" && role) {
        await governance.updateRole(workspaceId, role.id, {
          name: name.trim(),
          parent_id: selectedParentId || null,
        });
      } else if (mode === "copy" && role) {
        await governance.copyRole(workspaceId, role.id, name.trim());
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={title}>
      <form onSubmit={onSubmit} className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <label className="create-field">
          <span className="text-sm font-medium">角色名称</span>
          <input
            className="input"
            placeholder="请输入角色名称"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={mode === "edit" && Boolean(role && isProtectedRole(role.name))}
            required
          />
        </label>
        <label className="create-field">
          <span className="text-sm font-medium">父级角色</span>
          <select
            className="input"
            value={selectedParentId}
            onChange={(e) => setSelectedParentId(e.target.value)}
            disabled={mode === "copy"}
          >
            <option value="">无（根角色）</option>
            {parentOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {"—".repeat(option.depth)}
                {option.depth > 0 ? " " : ""}
                {option.label}
              </option>
            ))}
          </select>
        </label>
        {mode === "create" ? (
          <div>
            <p className="text-sm font-medium mb-2">功能权限</p>
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
          </div>
        ) : mode === "copy" ? (
          <p className="text-sm text-muted">
            将复制源角色的功能权限、菜单权限、API 权限与数据权限策略。
          </p>
        ) : null}
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button type="submit" className="btn" disabled={saving}>
            {saving ? "保存中…" : mode === "create" ? "创建" : "确定"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
