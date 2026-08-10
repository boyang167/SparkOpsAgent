"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";

interface WorkspaceSwitcherProps {
  workspaces: WorkspaceVM[];
  currentWorkspaceId?: string;
}

export function WorkspaceSwitcher({ workspaces, currentWorkspaceId }: WorkspaceSwitcherProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);

  async function onChange(workspaceId: string) {
    if (workspaceId === currentWorkspaceId) {
      return;
    }
    setLoading(true);
    try {
      await tenant.selectWorkspace(workspaceId);
      router.refresh();
    } finally {
      setLoading(false);
    }
  }

  if (workspaces.length === 0) {
    return <p className="text-sm text-[var(--muted)]">暂无 Workspace</p>;
  }

  return (
    <select
      className="select"
      value={currentWorkspaceId ?? workspaces[0]?.id ?? ""}
      disabled={loading}
      onChange={(e) => onChange(e.target.value)}
    >
      {workspaces.map((ws) => (
        <option key={ws.id} value={ws.id}>
          {ws.name}
        </option>
      ))}
    </select>
  );
}
