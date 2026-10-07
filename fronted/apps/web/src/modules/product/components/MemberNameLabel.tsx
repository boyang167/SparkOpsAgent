"use client";

import { useWorkspaceMembers } from "../hooks/useWorkspaceMembers";

interface MemberNameLabelProps {
  workspaceId: string;
  memberId?: string | null;
  memberName?: (memberId?: string | null) => string;
  className?: string;
  fallback?: string;
}

export function MemberNameLabel({
  workspaceId,
  memberId,
  memberName: memberNameProp,
  className = "text-sm text-muted-foreground",
  fallback = "—",
}: MemberNameLabelProps) {
  const { memberName: memberNameFromHook } = useWorkspaceMembers(memberNameProp ? "" : workspaceId);
  const resolveName = memberNameProp ?? memberNameFromHook;

  if (!memberId) {
    return <span className={className}>{fallback}</span>;
  }

  return (
    <span className={className} title={memberId}>
      {resolveName(memberId)}
    </span>
  );
}
