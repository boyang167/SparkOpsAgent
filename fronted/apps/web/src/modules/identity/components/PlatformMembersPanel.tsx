"use client";

import { useCallback, useEffect, useState } from "react";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PlatformMemberVM } from "@sparkops/contracts/identity";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface PlatformMembersPanelProps {
  currentMemberId: string;
}

export function PlatformMembersPanel({ currentMemberId }: PlatformMembersPanelProps) {
  const [members, setMembers] = useState<PlatformMemberVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [isSuperAdmin, setIsSuperAdmin] = useState(false);
  const [creating, setCreating] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const [resetMember, setResetMember] = useState<PlatformMemberVM | null>(null);
  const [newPassword, setNewPassword] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setMembers(await identity.listPlatformMembers());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError(null);
    try {
      await identity.createPlatformMember({
        email,
        password,
        display_name: displayName,
        is_super_admin: isSuperAdmin,
      });
      setCreateOpen(false);
      setEmail("");
      setPassword("");
      setDisplayName("");
      setIsSuperAdmin(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setCreating(false);
    }
  }

  async function toggleStatus(member: PlatformMemberVM) {
    if (member.id === currentMemberId) return;
    setActionId(member.id);
    setError(null);
    try {
      const next = member.status === "active" ? "disabled" : "active";
      await identity.updatePlatformMember(member.id, { status: next });
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新状态失败");
    } finally {
      setActionId(null);
    }
  }

  async function toggleSuperAdmin(member: PlatformMemberVM) {
    if (member.id === currentMemberId) return;
    setActionId(`sa:${member.id}`);
    setError(null);
    try {
      await identity.updatePlatformMember(member.id, { is_super_admin: !member.is_super_admin });
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新权限失败");
    } finally {
      setActionId(null);
    }
  }

  async function onResetPassword(e: React.FormEvent) {
    e.preventDefault();
    if (!resetMember || !newPassword) return;
    setActionId(`pwd:${resetMember.id}`);
    setError(null);
    try {
      await identity.resetPlatformMemberPassword(resetMember.id, { new_password: newPassword });
      setResetMember(null);
      setNewPassword("");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重置密码失败");
    } finally {
      setActionId(null);
    }
  }

  return (
    <div className="stack-lg">
      <Breadcrumb items={[{ label: "设置", href: "/settings/profile" }, { label: "平台用户" }]} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">平台用户管理</h1>
          <p className="text-sm text-muted mt-1">Super Admin 创建用户、重置密码、分配 Super Admin 权限</p>
        </div>
        <button type="button" className="btn" onClick={() => setCreateOpen(true)}>
          新建用户
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : members.length === 0 ? (
        <div className="empty-state">暂无用户</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>状态</th>
                <th>权限</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => {
                const isSelf = member.id === currentMemberId;
                return (
                  <tr key={member.id}>
                    <td>
                      <p className="font-medium">{member.display_name}</p>
                      <p className="text-xs text-muted">{member.email}</p>
                      {isSelf ? <p className="text-xs text-muted mt-0.5">当前登录</p> : null}
                    </td>
                    <td>
                      <span
                        className={`badge ${member.status === "active" ? "badge-primary" : "badge-warning"}`}
                      >
                        {member.status === "active" ? "正常" : "已停用"}
                      </span>
                    </td>
                    <td>
                      <span className={`badge ${member.is_super_admin ? "badge-primary" : ""}`}>
                        {member.is_super_admin ? "Super Admin" : "普通用户"}
                      </span>
                    </td>
                    <td className="text-sm text-muted">{new Date(member.created_at).toLocaleString()}</td>
                    <td>
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          disabled={isSelf || actionId === member.id}
                          onClick={() => toggleStatus(member)}
                        >
                          {member.status === "active" ? "停用" : "启用"}
                        </button>
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          disabled={isSelf || actionId === `sa:${member.id}`}
                          onClick={() => toggleSuperAdmin(member)}
                        >
                          {member.is_super_admin ? "取消 Super Admin" : "设为 Super Admin"}
                        </button>
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2 text-xs"
                          onClick={() => {
                            setResetMember(member);
                            setNewPassword("");
                          }}
                        >
                          重置密码
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="新建平台用户">
        <form onSubmit={onCreate} className="stack-md">
          <label className="stack-xs">
            <span className="label">邮箱</span>
            <input
              className="input"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">显示名称</span>
            <input
              className="input"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">初始密码</span>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              required
            />
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={isSuperAdmin}
              onChange={(e) => setIsSuperAdmin(e.target.checked)}
            />
            授予 Super Admin 权限
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={creating}>
              {creating ? "创建中…" : "创建"}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        open={Boolean(resetMember)}
        onClose={() => setResetMember(null)}
        title={resetMember ? `重置密码 · ${resetMember.email}` : "重置密码"}
      >
        <form onSubmit={onResetPassword} className="stack-md">
          <p className="text-sm text-muted">为用户设置新密码，至少 8 位。</p>
          <input
            className="input"
            type="password"
            placeholder="新密码"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            minLength={8}
            required
          />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setResetMember(null)}>
              取消
            </button>
            <button
              type="submit"
              className="btn"
              disabled={actionId === `pwd:${resetMember?.id}`}
            >
              确认重置
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
