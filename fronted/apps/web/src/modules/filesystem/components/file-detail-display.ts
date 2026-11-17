// Helpers for file detail drawer labels.

import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";

export function resolveMemberDisplayName(
  memberId: string,
  members: readonly WorkspaceMemberVM[] = [],
): string | null {
  const member = members.find((entry) => entry.member_id === memberId);
  if (!member) return null;
  const name = member.display_name.trim();
  return name || member.email.trim() || null;
}

export function formatFileNodeTimestamp(value?: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("zh-CN", { hour12: false });
}