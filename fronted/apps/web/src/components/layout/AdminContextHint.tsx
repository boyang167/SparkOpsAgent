"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import type { ConsoleKind } from "@/components/layout/nav-tree";

interface AdminContextHintProps {
  consoleKind: ConsoleKind;
  workspaces: WorkspaceVM[];
  currentWorkspaceId?: string;
}

export function AdminContextHint({
  consoleKind,
  workspaces,
  currentWorkspaceId,
}: AdminContextHintProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const current = workspaces.find((ws) => ws.id === currentWorkspaceId) ?? workspaces[0];

  async function onWorkspaceChange(workspaceId: string) {
    if (workspaceId === currentWorkspaceId || loading) return;
    setLoading(true);
    try {
      await tenant.selectWorkspace(workspaceId);
      router.refresh();
    } finally {
      setLoading(false);
    }
  }

  if (consoleKind === "platform") {
    return null;
  }

  if (!current) {
    return null;
  }

  if (workspaces.length <= 1) {
    return (
      <span className="admin-context-pill" title="当前工作区">
        {current.name}
      </span>
    );
  }

  return (
    <label className="admin-context-bar">
      <span className="sr-only">工作区</span>
      <select
        className="admin-context-select"
        value={current.id}
        disabled={loading}
        aria-label="切换工作区"
        onChange={(e) => void onWorkspaceChange(e.target.value)}
      >
        {workspaces.map((ws) => (
          <option key={ws.id} value={ws.id}>
            {ws.name}
          </option>
        ))}
      </select>
    </label>
  );
}
