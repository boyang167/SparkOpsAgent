"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PlatformMemberVM } from "@sparkops/contracts/identity";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface PlatformMembersPanelProps {
  currentMemberId: string;
}

type MemberForm = {
  email: string;
  password: string;
  display_name: string;
  status: "active" | "disabled";
  is_super_admin: boolean;
};

const PAGE_SIZES = [10, 30, 50, 100] as const;

const EMPTY_FORM: MemberForm = {
  email: "",
  password: "",
  display_name: "",
  status: "active",
  is_super_admin: false,
};

function generateRandomPassword(length = 12): string {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*";
  let password = "";
  for (let i = 0; i < length; i += 1) {
    password += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return password;
}

function shortId(id: string): string {
  return id.length > 8 ? `${id.slice(0, 8)}…` : id;
}

export function PlatformMembersPanel({ currentMemberId }: PlatformMembersPanelProps) {
  const [members, setMembers] = useState<PlatformMemberVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [actionId, setActionId] = useState<string | null>(null);

  const [searchEmail, setSearchEmail] = useState("");
  const [searchName, setSearchName] = useState("");
  const [appliedEmail, setAppliedEmail] = useState("");
  const [appliedName, setAppliedName] = useState("");

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(10);

  const [formMode, setFormMode] = useState<"add" | "edit" | null>(null);
  const [editingMember, setEditingMember] = useState<PlatformMemberVM | null>(null);
  const [form, setForm] = useState<MemberForm>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

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

  const filteredMembers = useMemo(() => {
    const emailQ = appliedEmail.trim().toLowerCase();
    const nameQ = appliedName.trim().toLowerCase();
    return members.filter((member) => {
      if (emailQ && !member.email.toLowerCase().includes(emailQ)) return false;
      if (nameQ && !member.display_name.toLowerCase().includes(nameQ)) return false;
      return true;
    });
  }, [members, appliedEmail, appliedName]);

  const total = filteredMembers.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(page, totalPages);

  const pageItems = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredMembers.slice(start, start + pageSize);
  }, [filteredMembers, currentPage, pageSize]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3000);
  }

  function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setAppliedEmail(searchEmail);
    setAppliedName(searchName);
    setPage(1);
  }

  function onResetSearch() {
    setSearchEmail("");
    setSearchName("");
    setAppliedEmail("");
    setAppliedName("");
    setPage(1);
  }

  function openCreate() {
    setFormMode("add");
    setEditingMember(null);
    setForm(EMPTY_FORM);
  }

  function openEdit(member: PlatformMemberVM) {
    setFormMode("edit");
    setEditingMember(member);
    setForm({
      email: member.email,
      password: "",
      display_name: member.display_name,
      status: member.status,
      is_super_admin: member.is_super_admin,
    });
  }

  function closeForm() {
    setFormMode(null);
    setEditingMember(null);
    setForm(EMPTY_FORM);
  }

  async function onSaveForm(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      if (formMode === "add") {
        await identity.createPlatformMember({
          email: form.email,
          password: form.password,
          display_name: form.display_name,
          is_super_admin: form.is_super_admin,
        });
        flashSuccess("用户创建成功");
      } else if (formMode === "edit" && editingMember) {
        await identity.updatePlatformMember(editingMember.id, {
          display_name: form.display_name,
          status: form.status,
          is_super_admin: form.is_super_admin,
        });
        flashSuccess("用户信息已更新");
      }
      closeForm();
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function switchStatus(member: PlatformMemberVM, enabled: boolean) {
    if (member.id === currentMemberId) return;
    const next = enabled ? "active" : "disabled";
    if (member.status === next) return;
    setActionId(member.id);
    setError(null);
    try {
      await identity.updatePlatformMember(member.id, { status: next });
      flashSuccess(enabled ? "已启用用户" : "已停用用户");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新状态失败");
    } finally {
      setActionId(null);
    }
  }

  async function switchSuperAdmin(member: PlatformMemberVM, grant: boolean) {
    if (member.id === currentMemberId) return;
    if (member.is_super_admin === grant) return;
    setActionId(`sa:${member.id}`);
    setError(null);
    try {
      await identity.updatePlatformMember(member.id, { is_super_admin: grant });
      flashSuccess(grant ? "已授予 Super Admin" : "已取消 Super Admin");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新权限失败");
    } finally {
      setActionId(null);
    }
  }

  async function onDelete(member: PlatformMemberVM) {
    if (member.id === currentMemberId) return;
    if (!window.confirm(`确定删除用户「${member.display_name}」（${member.email}）？此操作为软删除。`)) return;
    setActionId(`del:${member.id}`);
    setError(null);
    try {
      await identity.deletePlatformMember(member.id);
      flashSuccess("用户已删除");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
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
      flashSuccess("密码重置成功");
      setResetMember(null);
      setNewPassword("");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "重置密码失败");
    } finally {
      setActionId(null);
    }
  }

  async function onGeneratePassword() {
    const password = generateRandomPassword();
    setNewPassword(password);
    try {
      await navigator.clipboard.writeText(password);
      flashSuccess("随机密码已生成并复制到剪贴板");
    } catch {
      flashSuccess("随机密码已生成");
    }
  }

  const formFooter = (
    <div className="flex justify-end gap-2">
      <button type="button" className="btn-secondary" onClick={closeForm}>
        取消
      </button>
      <button type="submit" className="btn" form="member-form" disabled={saving}>
        {saving ? "保存中…" : "确定"}
      </button>
    </div>
  );

  return (
    <div className="stack-lg">
      <Breadcrumb items={[{ label: "设置", href: "/settings/profile" }, { label: "平台用户" }]} />

      <div>
        <h1 className="text-xl font-semibold">用户管理</h1>
        <p className="text-sm text-muted mt-1">
          管理平台全局账号：查询、新建、编辑、启停、Super Admin 权限与密码重置
        </p>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <section className="card p-4">
        <form onSubmit={onSearch} className="flex flex-wrap items-end gap-3">
          <label className="stack-xs min-w-[180px] flex-1">
            <span className="label">显示名称</span>
            <input
              className="input"
              placeholder="昵称"
              value={searchName}
              onChange={(e) => setSearchName(e.target.value)}
            />
          </label>
          <label className="stack-xs min-w-[220px] flex-1">
            <span className="label">邮箱</span>
            <input
              className="input"
              type="email"
              placeholder="搜索用户邮箱"
              value={searchEmail}
              onChange={(e) => setSearchEmail(e.target.value)}
            />
          </label>
          <div className="flex gap-2">
            <button type="submit" className="btn">
              查询
            </button>
            <button type="button" className="btn-secondary" onClick={onResetSearch}>
              重置
            </button>
          </div>
        </form>
      </section>

      <section className="card p-4 stack-md">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-medium">用户列表</h2>
          <button type="button" className="btn" onClick={openCreate}>
            新增用户
          </button>
        </div>

        {loading ? (
          <div className="empty-state">加载中…</div>
        ) : pageItems.length === 0 ? (
          <div className="empty-state">{total === 0 ? "暂无用户" : "无匹配用户"}</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>显示名称</th>
                  <th>邮箱</th>
                  <th>Super Admin</th>
                  <th>启用</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((member) => {
                  const isSelf = member.id === currentMemberId;
                  const busy = actionId === member.id || actionId === `sa:${member.id}` || actionId === `del:${member.id}`;
                  return (
                    <tr key={member.id}>
                      <td className="font-mono text-xs text-muted" title={member.id}>
                        {shortId(member.id)}
                      </td>
                      <td>
                        <p className="font-medium">{member.display_name}</p>
                        {isSelf ? <p className="text-xs text-muted">当前登录</p> : null}
                      </td>
                      <td className="text-sm">{member.email}</td>
                      <td>
                        <label className="inline-flex items-center gap-2 text-sm">
                          <input
                            type="checkbox"
                            className="h-4 w-4"
                            checked={member.is_super_admin}
                            disabled={isSelf || busy}
                            onChange={(e) => void switchSuperAdmin(member, e.target.checked)}
                          />
                          <span className={member.is_super_admin ? "text-primary font-medium" : "text-muted"}>
                            {member.is_super_admin ? "是" : "否"}
                          </span>
                        </label>
                      </td>
                      <td>
                        <label className="inline-flex items-center gap-2 text-sm">
                          <input
                            type="checkbox"
                            role="switch"
                            className="h-4 w-4"
                            checked={member.status === "active"}
                            disabled={isSelf || busy}
                            onChange={(e) => void switchStatus(member, e.target.checked)}
                          />
                          <span className={member.status === "active" ? "text-emerald-700" : "text-amber-700"}>
                            {member.status === "active" ? "启用" : "停用"}
                          </span>
                        </label>
                      </td>
                      <td className="text-sm text-muted whitespace-nowrap">
                        {new Date(member.created_at).toLocaleString()}
                      </td>
                      <td>
                        <div className="flex flex-wrap gap-x-3 gap-y-1 text-sm">
                          <button
                            type="button"
                            className="text-primary hover:underline"
                            onClick={() => openEdit(member)}
                          >
                            编辑
                          </button>
                          <button
                            type="button"
                            className="text-primary hover:underline"
                            onClick={() => {
                              setResetMember(member);
                              setNewPassword("");
                            }}
                          >
                            重置密码
                          </button>
                          <button
                            type="button"
                            className="text-red-600 hover:underline disabled:opacity-50"
                            disabled={isSelf || busy}
                            onClick={() => void onDelete(member)}
                          >
                            删除
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

        {!loading && total > 0 ? (
          <div className="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-[var(--border)]">
            <p className="text-sm text-muted">共 {total} 条</p>
            <div className="flex flex-wrap items-center gap-3">
              <label className="flex items-center gap-2 text-sm text-muted">
                每页
                <select
                  className="input !py-1 !px-2 !w-auto"
                  value={pageSize}
                  onChange={(e) => {
                    setPageSize(Number(e.target.value));
                    setPage(1);
                  }}
                >
                  {PAGE_SIZES.map((size) => (
                    <option key={size} value={size}>
                      {size}
                    </option>
                  ))}
                </select>
              </label>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  className="btn-secondary !py-1 !px-2 text-xs"
                  disabled={currentPage <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  上一页
                </button>
                <span className="text-sm text-muted">
                  {currentPage} / {totalPages}
                </span>
                <button
                  type="button"
                  className="btn-secondary !py-1 !px-2 text-xs"
                  disabled={currentPage >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  下一页
                </button>
              </div>
            </div>
          </div>
        ) : null}
      </section>

      <Modal
        open={formMode !== null}
        onClose={closeForm}
        title={formMode === "add" ? "新增用户" : "编辑用户"}
        size="lg"
        footer={formFooter}
      >
        <form id="member-form" onSubmit={onSaveForm} className="stack-md">
          {formMode === "add" ? (
            <>
              <label className="stack-xs">
                <span className="label">邮箱</span>
                <input
                  className="input"
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
                  required
                />
              </label>
              <label className="stack-xs">
                <span className="label">初始密码</span>
                <input
                  className="input"
                  type="password"
                  value={form.password}
                  onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
                  minLength={8}
                  required
                />
              </label>
            </>
          ) : (
            <label className="stack-xs">
              <span className="label">邮箱</span>
              <input className="input" type="email" value={form.email} disabled />
            </label>
          )}
          <label className="stack-xs">
            <span className="label">显示名称</span>
            <input
              className="input"
              value={form.display_name}
              onChange={(e) => setForm((f) => ({ ...f, display_name: e.target.value }))}
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">状态</span>
            <select
              className="input"
              value={form.status}
              disabled={formMode === "edit" && editingMember?.id === currentMemberId}
              onChange={(e) =>
                setForm((f) => ({ ...f, status: e.target.value as MemberForm["status"] }))
              }
            >
              <option value="active">启用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.is_super_admin}
              disabled={formMode === "edit" && editingMember?.id === currentMemberId}
              onChange={(e) => setForm((f) => ({ ...f, is_super_admin: e.target.checked }))}
            />
            授予 Super Admin 权限
          </label>
        </form>
      </Modal>

      <Modal
        open={Boolean(resetMember)}
        onClose={() => setResetMember(null)}
        title={resetMember ? `重置密码 · ${resetMember.display_name}` : "重置密码"}
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setResetMember(null)}>
              取消
            </button>
            <button
              type="submit"
              form="reset-password-form"
              className="btn"
              disabled={actionId === `pwd:${resetMember?.id}`}
            >
              确定
            </button>
          </div>
        }
      >
        <form id="reset-password-form" onSubmit={onResetPassword} className="stack-md">
          <label className="stack-xs">
            <span className="label">用户账号</span>
            <input className="input" value={resetMember?.email ?? ""} disabled />
          </label>
          <label className="stack-xs">
            <span className="label">显示名称</span>
            <input className="input" value={resetMember?.display_name ?? ""} disabled />
          </label>
          <label className="stack-xs">
            <span className="label">新密码</span>
            <div className="flex gap-2">
              <input
                className="input flex-1"
                type="password"
                placeholder="至少 8 位"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                minLength={8}
                required
              />
              <button type="button" className="btn-secondary whitespace-nowrap" onClick={() => void onGeneratePassword()}>
                生成随机密码
              </button>
            </div>
          </label>
        </form>
      </Modal>
    </div>
  );
}
