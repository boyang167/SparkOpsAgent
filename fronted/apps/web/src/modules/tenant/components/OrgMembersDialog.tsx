"use client";

import { useCallback, useEffect, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { OrgMemberVM } from "@sparkops/contracts/tenant";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformAuditLogsPanel } from "./PlatformAuditLogsPanel";
import { OrgQuotaPanel } from "./OrgQuotaPanel";

const ROLE_LABELS: Record<string, string> = {
  owner: "所有者",
  admin: "管理员",
  member: "成员",
};

type Tab = "members" | "audit" | "transfer" | "quota";

interface OrgMembersDialogProps {
  open: boolean;
  orgId: string;
  orgName: string;
  onClose: () => void;
}

export function OrgMembersDialog({ open, orgId, orgName, onClose }: OrgMembersDialogProps) {
  const [tab, setTab] = useState<Tab>("members");
  const [members, setMembers] = useState<OrgMemberVM[]>([]);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [myRole, setMyRole] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"admin" | "member">("member");
  const [inviting, setInviting] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const [transferEmail, setTransferEmail] = useState("");
  const [transferring, setTransferring] = useState(false);

  const canManage = permissions.includes("org:member:manage");
  const canRead = permissions.includes("org:read") || canManage;
  const isOwner = myRole === "owner";

  const load = useCallback(async () => {
    if (!orgId) return;
    setLoading(true);
    setError(null);
    try {
      const [org, memberItems, perms] = await Promise.all([
        tenant.getOrganization(orgId),
        tenant.listOrgMembers(orgId),
        tenant.getMyOrgPermissions(orgId),
      ]);
      setMembers(memberItems);
      setPermissions(perms);
      setMyRole(org.my_role ?? "");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载成员失败");
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  useEffect(() => {
    if (!open) return;
    setTab("members");
    setEmail("");
    setInviteRole("member");
    setTransferEmail("");
    setError(null);
    void load();
  }, [open, load]);

  async function onInvite(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setInviting(true);
    setError(null);
    try {
      await tenant.inviteOrgMember(orgId, { email: email.trim(), role: inviteRole });
      setEmail("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "邀请失败");
    } finally {
      setInviting(false);
    }
  }

  async function onChangeRole(memberId: string, role: "admin" | "member") {
    setActionId(memberId);
    setError(null);
    try {
      await tenant.updateOrgMemberRole(orgId, memberId, role);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新角色失败");
    } finally {
      setActionId(null);
    }
  }

  async function onRemove(memberId: string) {
    setActionId(`remove:${memberId}`);
    setError(null);
    try {
      await tenant.removeOrgMember(orgId, memberId);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "移除失败");
    } finally {
      setActionId(null);
    }
  }

  async function onTransfer(e: React.FormEvent) {
    e.preventDefault();
    if (!transferEmail.trim()) return;
    setTransferring(true);
    setError(null);
    try {
      await tenant.transferOrgOwner(orgId, transferEmail.trim());
      setTransferEmail("");
      await load();
      setTab("members");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "转让失败");
    } finally {
      setTransferring(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`${orgName} · 组织管理`} size="lg">
      <div className="stack-md">
        <div className="flex gap-2 border-b border-[var(--border)] pb-2">
          <button
            type="button"
            className={`text-sm px-3 py-1 rounded-md ${tab === "members" ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-muted"}`}
            onClick={() => setTab("members")}
          >
            成员
          </button>
          {canManage ? (
            <button
              type="button"
              className={`text-sm px-3 py-1 rounded-md ${tab === "audit" ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-muted"}`}
              onClick={() => setTab("audit")}
            >
              审计
            </button>
          ) : null}
          {canRead ? (
            <button
              type="button"
              className={`text-sm px-3 py-1 rounded-md ${tab === "quota" ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-muted"}`}
              onClick={() => setTab("quota")}
            >
              配额
            </button>
          ) : null}
          {isOwner ? (
            <button
              type="button"
              className={`text-sm px-3 py-1 rounded-md ${tab === "transfer" ? "bg-[var(--accent-soft)] text-[var(--accent)]" : "text-muted"}`}
              onClick={() => setTab("transfer")}
            >
              转让 Owner
            </button>
          ) : null}
        </div>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {tab === "members" ? (
          <>
            {canManage ? (
              <form onSubmit={onInvite} className="tenant-invite-form">
                <div className="flex flex-col sm:flex-row gap-2">
                  <input
                    className="input flex-1"
                    type="email"
                    placeholder="成员邮箱（须已注册）"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                  />
                  <select
                    className="input sm:w-32"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value as "admin" | "member")}
                  >
                    <option value="member">成员</option>
                    <option value="admin">管理员</option>
                  </select>
                  <button type="submit" className="btn shrink-0" disabled={inviting}>
                    {inviting ? "邀请中…" : "邀请"}
                  </button>
                </div>
              </form>
            ) : null}
            {loading ? (
              <div className="empty-state">加载中…</div>
            ) : members.length === 0 ? (
              <div className="empty-state">暂无组织成员</div>
            ) : (
              <div className="table-wrap">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>成员</th>
                      <th>角色</th>
                      {canManage ? <th>操作</th> : null}
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
                          <span className="badge badge-neutral">{ROLE_LABELS[member.role] ?? member.role}</span>
                        </td>
                        {canManage ? (
                          <td>
                            {member.role === "owner" ? (
                              <span className="text-xs text-muted">—</span>
                            ) : (
                              <div className="flex flex-wrap gap-2">
                                {member.role !== "admin" ? (
                                  <button
                                    type="button"
                                    className="btn-secondary !py-1 !px-2 text-xs"
                                    disabled={actionId === member.member_id}
                                    onClick={() => onChangeRole(member.member_id, "admin")}
                                  >
                                    设为管理员
                                  </button>
                                ) : null}
                                {member.role !== "member" ? (
                                  <button
                                    type="button"
                                    className="btn-secondary !py-1 !px-2 text-xs"
                                    disabled={actionId === member.member_id}
                                    onClick={() => onChangeRole(member.member_id, "member")}
                                  >
                                    设为成员
                                  </button>
                                ) : null}
                                <button
                                  type="button"
                                  className="btn-secondary !py-1 !px-2 text-xs text-red-600"
                                  disabled={actionId === `remove:${member.member_id}`}
                                  onClick={() => onRemove(member.member_id)}
                                >
                                  移除
                                </button>
                              </div>
                            )}
                          </td>
                        ) : null}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        ) : null}

        {tab === "audit" && canManage ? (
          <PlatformAuditLogsPanel orgId={orgId} title="组织审计日志" />
        ) : null}

        {tab === "quota" && canRead ? (
          <OrgQuotaPanel orgId={orgId} canEdit={canManage} />
        ) : null}

        {tab === "transfer" && isOwner ? (
          <form onSubmit={onTransfer} className="stack-md">
            <p className="text-sm text-muted">
              转让后您将降为管理员。新 Owner 须已在平台注册；若不在本组织会自动加入。
            </p>
            <input
              className="input"
              type="email"
              placeholder="新 Owner 邮箱"
              value={transferEmail}
              onChange={(e) => setTransferEmail(e.target.value)}
              required
            />
            <button type="submit" className="btn w-fit" disabled={transferring}>
              {transferring ? "转让中…" : "确认转让 Owner"}
            </button>
          </form>
        ) : null}
      </div>
    </Modal>
  );
}
