"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";

function memberLabel(member: WorkspaceMemberVM): string {
  return member.display_name?.trim() || member.email || member.member_id;
}

export function useWorkspaceMembers(workspaceId: string) {
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!workspaceId) {
      setMembers([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    void governance
      .listMembers(workspaceId)
      .then((items) => {
        if (!cancelled) setMembers(items);
      })
      .catch(() => {
        if (!cancelled) setMembers([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  const memberById = useMemo(() => {
    const map = new Map<string, WorkspaceMemberVM>();
    for (const member of members) {
      map.set(member.member_id, member);
    }
    return map;
  }, [members]);

  const memberName = useCallback(
    (memberId?: string | null): string => {
      if (!memberId) return "—";
      const member = memberById.get(memberId);
      if (member) return memberLabel(member);
      return memberId.length > 8 ? `${memberId.slice(0, 8)}…` : memberId;
    },
    [memberById],
  );

  return { members, loading, memberName };
}
