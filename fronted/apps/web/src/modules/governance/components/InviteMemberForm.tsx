"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

interface InviteMemberFormProps {
  workspaceId: string;
}

export function InviteMemberForm({ workspaceId }: InviteMemberFormProps) {
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

  return (
    <form className="flex flex-col sm:flex-row gap-2" onSubmit={onSubmit}>
      <input
        className="input flex-1"
        type="email"
        placeholder="成员邮箱（需已注册）"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
      />
      <button className="btn shrink-0" type="submit" disabled={loading || !email.trim()}>
        {loading ? "邀请中..." : "邀请成员"}
      </button>
      {error ? <p className="error sm:col-span-2 w-full">{error}</p> : null}
    </form>
  );
}
