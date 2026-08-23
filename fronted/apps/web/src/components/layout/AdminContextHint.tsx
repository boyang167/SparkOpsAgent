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
    return <span className="topbar-hint hidden sm:inline">平台运营 · 跨租户治理</span>;
  }

  return (
    <div className="context-bar">
      <span className="topbar-hint hidden sm:inline">IAM 上下文</span>
      <span className="context-bar-sep hidden sm:inline" aria-hidden>
        /
      </span>
      <label className="context-bar-field">
        <span className="context-bar-label">工作区</span>
        <select
          className="context-bar-select"
          value={current?.id ?? ""}
          disabled={loading || workspaces.length <= 1}
          onChange={(e) => void onWorkspaceChange(e.target.value)}
        >
          {workspaces.map((ws) => (
            <option key={ws.id} value={ws.id}>
              {ws.name}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
