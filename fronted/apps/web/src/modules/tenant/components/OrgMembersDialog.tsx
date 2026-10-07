"use client";

import { useCallback, useEffect, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { OrgMemberVM } from "@sparkops/contracts/tenant";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MemberInviteSearchInput } from "@/modules/governance/components/MemberInviteSearchInput";
import { formatInviteError } from "@/modules/governance/components/invite-member-utils";
import { PlatformAuditLogsPanel } from "./PlatformAuditLogsPanel";
import { OrgQuotaPanel } from "./OrgQuotaPanel";

const ROLE_LABELS: Record<string, string> = {
  owner: "所有者",
  admin: "管理员",
  member: "成员",
};

type Tab = "members" | "audit" | "quota";

interface OrgMembersDialogProps {
  open: boolean;
  orgId: string;
  orgName: string;
  onClose: () => void;
}

function MemberAvatar({ name, email }: { name: string; email: string }) {
  const initial = (name || email).charAt(0).toUpperCase();
  return (
    <span className="org-member-avatar" aria-hidden>
      {initial}
    </span>
  );
}

export function OrgMembersDialog({ open, orgId, orgName, onClose }: OrgMembersDialogProps) {
  const [tab, setTab] = useState<Tab>("members");
  const [members, setMembers] = useState<OrgMemberVM[]>([]);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [myRole, setMyRole] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // invite state
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [inviteSuccess, setInviteSuccess] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"admin" | "member">("member");
  const [inviting, setInviting] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);

  // transfer state
  const [transferEmail, setTransferEmail] = useState("");
  const [transferring, setTransferring] = useState(false);
  const [showTransfer, setShowTransfer] = useState(false);

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
    setShowTransfer(false);
    setError(null);
    setInviteError(null);
    setInviteSuccess(null);
    void load();
  }, [open, load]);

  async function onInvite(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setInviting(true);
    setInviteError(null);
    setInviteSuccess(null);
    try {
      const invited = await tenant.inviteOrgMember(orgId, { email: email.trim(), role: inviteRole });
      setEmail("");
      setInviteSuccess(`已邀请 ${invited.display_name || invited.email} 为${ROLE_LABELS[invited.role] ?? invited.role}`);
      await load();
    } catch (err) {
      setInviteError(formatInviteError(err));
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
      setShowTransfer(false);
      await load();
      setTab("members");
    } catch (err) {
      setError(err instanceof BffError ? err.message : "转让失败");
    } finally {
      setTransferring(false);
    }
  }

  const tabs: { key: Tab; label: string; visible: boolean }[] = [
    { key: "members", label: "成员", visible: true },
    { key: "quota", label: "配额", visible: canRead },
    { key: "audit", label: "审计", visible: canManage },
  ];

  return (
    <Modal open={open} onClose={onClose} title={`${orgName} · 组织管理`} size="lg">
      <div className="org-manage">
        {/* ── Segmented Tab Control ── */}
        <nav className="org-manage-tabs" aria-label="组织管理导航">
          {tabs.filter((t) => t.visible).map((t) => (
            <button
              key={t.key}
              type="button"
              className={`org-manage-tab${tab === t.key ? " is-active" : ""}`}
              onClick={() => setTab(t.key)}
              aria-current={tab === t.key ? "page" : undefined}
            >
              {t.label}
            </button>
          ))}
        </nav>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {/* ── Members Panel ── */}
        {tab === "members" ? (
          <div className="org-manage-panel">
            {/* Invite inline bar */}
            {canManage ? (
              <form onSubmit={onInvite} className="org-invite">
                <div className="org-invite-row">
                  <MemberInviteSearchInput
                    value={email}
                    onChange={(value) => {
                      setEmail(value);
                      setInviteError(null);
                      setInviteSuccess(null);
                    }}
                    variant="org"
                    placeholder="搜索邮箱或姓名，邀请加入组织…"
                    disabled={inviting}
                    searchCandidates={(query, limit) => tenant.searchOrgMemberCandidates(orgId, query, limit)}
                  />
                  <select
                    className="org-invite-role"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value as "admin" | "member")}
                    disabled={inviting}
                  >
                    <option value="member">成员</option>
                    <option value="admin">管理员</option>
                  </select>
                  <button type="submit" className="org-invite-btn" disabled={inviting || !email.trim()}>
                    {inviting ? "邀请中…" : "邀请"}
                  </button>
                </div>
                {inviteError ? <InlineAlert tone="error">{inviteError}</InlineAlert> : null}
                {inviteSuccess ? <InlineAlert tone="success">{inviteSuccess}</InlineAlert> : null}
              </form>
            ) : null}

            {/* Member list */}
            {loading ? (
              <div className="org-member-skeleton">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="org-member-skeleton-row" />
                ))}
              </div>
            ) : members.length === 0 ? (
              <div className="org-empty">
                <p>暂无组织成员</p>
              </div>
            ) : (
              <ul className="org-member-list">
                {members.map((member) => (
                  <li key={member.member_id} className="org-member-item">
                    <MemberAvatar name={member.display_name || ""} email={member.email} />
                    <div className="org-member-info">
                      <span className="org-member-name">{member.display_name || member.email}</span>
                      {member.display_name ? (
                        <span className="org-member-email">{member.email}</span>
                      ) : null}
                    </div>
                    <span className={`org-member-role${member.role === "owner" ? " is-owner" : ""}`}>
                      {ROLE_LABELS[member.role] ?? member.role}
                    </span>
                    {canManage && member.role !== "owner" ? (
                      <div className="org-member-actions">
                        {member.role !== "admin" ? (
                          <button
                            type="button"
                            className="org-member-action"
                            disabled={actionId === member.member_id}
                            onClick={() => onChangeRole(member.member_id, "admin")}
                          >
                            设为管理员
                          </button>
                        ) : (
                          <button
                            type="button"
                            className="org-member-action"
                            disabled={actionId === member.member_id}
                            onClick={() => onChangeRole(member.member_id, "member")}
                          >
                            设为成员
                          </button>
                        )}
                        <button
                          type="button"
                          className="org-member-action is-danger"
                          disabled={actionId === `remove:${member.member_id}`}
                          onClick={() => onRemove(member.member_id)}
                        >
                          移除
                        </button>
                      </div>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}

            {/* Transfer Owner — danger zone, collapsed by default */}
            {isOwner ? (
              <div className="org-danger-zone">
                {!showTransfer ? (
                  <button
                    type="button"
                    className="org-danger-trigger"
                    onClick={() => setShowTransfer(true)}
                  >
                    <svg viewBox="0 0 20 20" fill="currentColor" width="14" height="14" aria-hidden>
                      <path fillRule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.168 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clipRule="evenodd" />
                    </svg>
                    转让 Owner
                  </button>
                ) : (
                  <form onSubmit={onTransfer} className="org-transfer-form">
                    <p className="org-transfer-warn">
                      转让后您将降为管理员，此操作不可撤销。新 Owner 须已在平台注册。
                    </p>
                    <div className="org-transfer-row">
                      <input
                        className="org-transfer-input"
                        type="email"
                        placeholder="新 Owner 邮箱"
                        value={transferEmail}
                        onChange={(e) => setTransferEmail(e.target.value)}
                        required
                        autoFocus
                      />
                      <button type="submit" className="org-transfer-btn" disabled={transferring}>
                        {transferring ? "转让中…" : "确认转让"}
                      </button>
                      <button
                        type="button"
                        className="org-transfer-cancel"
                        onClick={() => { setShowTransfer(false); setTransferEmail(""); }}
                      >
                        取消
                      </button>
                    </div>
                  </form>
                )}
              </div>
            ) : null}
          </div>
        ) : null}

        {/* ── Quota Panel ── */}
        {tab === "quota" && canRead ? (
          <div className="org-manage-panel">
            <OrgQuotaPanel orgId={orgId} canEdit={canManage} />
          </div>
        ) : null}

        {/* ── Audit Panel ── */}
        {tab === "audit" && canManage ? (
          <div className="org-manage-panel">
            <PlatformAuditLogsPanel orgId={orgId} title="组织审计日志" showBreadcrumb={false} />
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
