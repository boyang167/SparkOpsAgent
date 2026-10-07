"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { MemberInviteSearchInput } from "@/modules/governance/components/MemberInviteSearchInput";
import { formatInviteError } from "@/modules/governance/components/invite-member-utils";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface InviteMemberFormProps {
  workspaceId: string;
  variant?: "default" | "admin";
}

export function InviteMemberForm({ workspaceId, variant = "default" }: InviteMemberFormProps) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSuccess(null);
    try {
      const invited = await governance.inviteMember(workspaceId, email.trim());
      setEmail("");
      setSuccess(`已邀请 ${invited.display_name || invited.email}`);
      router.refresh();
    } catch (err) {
      setError(formatInviteError(err));
    } finally {
      setLoading(false);
    }
  }

  if (variant === "admin") {
    return (
      <form className="admin-members-invite" onSubmit={onSubmit}>
        <MemberInviteSearchInput
          workspaceId={workspaceId}
          value={email}
          onChange={setEmail}
          variant="admin"
          placeholder="搜索邮箱或姓名（须已注册）"
          disabled={loading}
        />
        <button className="btn admin-members-invite-btn" type="submit" disabled={loading || !email.trim()}>
          {loading ? "邀请中…" : "邀请成员"}
        </button>
        {error ? <p className="admin-members-invite-error">{error}</p> : null}
        {success ? <p className="admin-members-invite-success">{success}</p> : null}
      </form>
    );
  }

  return (
    <form className="stack-sm" onSubmit={onSubmit}>
      <p className="text-sm text-muted">搜索并选择已注册的平台用户加入当前工作区。</p>
      <div className="flex flex-col sm:flex-row gap-2 sm:items-center">
        <MemberInviteSearchInput
          workspaceId={workspaceId}
          value={email}
          onChange={setEmail}
          placeholder="搜索邮箱或姓名（须已注册）"
          disabled={loading}
        />
        <button className="btn shrink-0" type="submit" disabled={loading || !email.trim()}>
          {loading ? "邀请中…" : "邀请成员"}
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}
    </form>
  );
}
