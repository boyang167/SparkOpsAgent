"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { MemberInviteSearchInput } from "@/modules/governance/components/MemberInviteSearchInput";

interface InviteMemberFormProps {
  workspaceId: string;
  variant?: "default" | "admin";
}

export function InviteMemberForm({ workspaceId, variant = "default" }: InviteMemberFormProps) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await governance.inviteMember(workspaceId, email.trim());
      setEmail("");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "邀请失败");
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
          placeholder="搜索邮箱或姓名（需已注册）"
          disabled={loading}
        />
        <button className="btn admin-members-invite-btn" type="submit" disabled={loading || !email.trim()}>
          {loading ? "邀请中…" : "邀请成员"}
        </button>
        {error ? <p className="admin-members-invite-error">{error}</p> : null}
      </form>
    );
  }

  return (
    <form className="flex flex-col sm:flex-row gap-2" onSubmit={onSubmit}>
      <MemberInviteSearchInput
        workspaceId={workspaceId}
        value={email}
        onChange={setEmail}
        placeholder="搜索邮箱或姓名（需已注册）"
        disabled={loading}
      />
      <button className="btn shrink-0" type="submit" disabled={loading || !email.trim()}>
        {loading ? "邀请中..." : "邀请成员"}
      </button>
      {error ? <p className="error sm:col-span-2 w-full">{error}</p> : null}
    </form>
  );
}
