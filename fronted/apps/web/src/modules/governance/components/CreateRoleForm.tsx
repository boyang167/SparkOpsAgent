"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";

interface CreateRoleFormProps {
  workspaceId: string;
}

export function CreateRoleForm({ workspaceId }: CreateRoleFormProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [permissions, setPermissions] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  function togglePermission(value: string) {
    setPermissions((prev) =>
      prev.includes(value) ? prev.filter((p) => p !== value) : [...prev, value],
    );
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await governance.createRole(workspaceId, { name, permissions });
      setOpen(false);
      setName("");
      setPermissions([]);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setLoading(false);
    }
  }

  if (!open) {
    return (
      <button className="btn" type="button" onClick={() => setOpen(true)}>
        新建角色
      </button>
    );
  }

  return (
    <form className="form-panel space-y-4" onSubmit={onSubmit}>
      <div className="flex items-center justify-between gap-4">
        <h3 className="section-title">新建角色</h3>
        <button className="btn-ghost !py-1 !px-2" type="button" onClick={() => setOpen(false)}>
          取消
        </button>
      </div>
      <div>
        <label className="label" htmlFor="role-name">角色名称</label>
        <input
          id="role-name"
          className="input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="例如 editor"
          required
        />
      </div>
      <div>
        <p className="label">权限</p>
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
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn" type="submit" disabled={loading || permissions.length === 0}>
        {loading ? "创建中..." : "创建角色"}
      </button>
    </form>
  );
}
