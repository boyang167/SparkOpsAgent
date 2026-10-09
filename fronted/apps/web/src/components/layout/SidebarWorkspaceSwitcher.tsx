"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { tenant } from "@sparkops/web-sdk";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { isOrgReadOnly, isWorkspaceReadOnly } from "@/modules/tenant/lib/org-display";

interface SidebarWorkspaceSwitcherProps {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
  currentWorkspaceId?: string;
}

export function SidebarWorkspaceSwitcher({
  organizations,
  workspaces,
  currentWorkspaceId,
}: SidebarWorkspaceSwitcherProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const current = useMemo(
    () =>
      workspaces.find((ws) => ws.id === currentWorkspaceId) ?? workspaces[0],
    [workspaces, currentWorkspaceId],
  );

  const currentOrg = useMemo(
    () => organizations.find((o) => o.id === current?.org_id),
    [organizations, current],
  );

  useEffect(() => {
    if (!open) return;
    function onPointerDown(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  async function switchWorkspace(workspaceId: string) {
    if (workspaceId === currentWorkspaceId || loading) return;
    const target = workspaces.find((ws) => ws.id === workspaceId);
    if (!target || isWorkspaceReadOnly(target)) return;
    setLoading(true);
    setOpen(false);
    try {
      await tenant.selectWorkspace(workspaceId);
      router.refresh();
    } finally {
      setLoading(false);
    }
  }

  if (!current) return null;

  const showMultiple = workspaces.filter((ws) => !isWorkspaceReadOnly(ws)).length > 1;
  const orgLabel = currentOrg?.name;
  const showOrg = organizations.length > 0 && orgLabel;

  return (
    <div className="sidebar-ws" ref={rootRef}>
      <button
        type="button"
        className="sidebar-ws-trigger"
        onClick={() => showMultiple && setOpen((v) => !v)}
        aria-haspopup={showMultiple ? "listbox" : undefined}
        aria-expanded={showMultiple ? open : undefined}
        disabled={loading}
      >
        <span className="sidebar-ws-icon" aria-hidden>
          {current.name.charAt(0).toUpperCase()}
        </span>
        <span className="sidebar-ws-copy">
          <strong className="sidebar-ws-name">{current.name}</strong>
          {showOrg ? <span className="sidebar-ws-org">{orgLabel}</span> : null}
        </span>
        {showMultiple ? (
          <svg className="sidebar-ws-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <path d="M5 6l3-3 3 3M5 10l3 3 3-3" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        ) : null}
      </button>

      {open ? (
        <div className="sidebar-ws-menu" role="listbox" aria-label="切换工作区">
          {workspaces.map((ws) => {
            const active = ws.id === currentWorkspaceId;
            const disabled = isWorkspaceReadOnly(ws);
            const wsOrg = organizations.find((o) => o.id === ws.org_id);
            return (
              <button
                key={ws.id}
                type="button"
                role="option"
                aria-selected={active}
                className={`sidebar-ws-option${active ? " is-active" : ""}`}
                disabled={disabled || loading}
                onClick={() => void switchWorkspace(ws.id)}
              >
                <span className="sidebar-ws-option-icon">
                  {ws.name.charAt(0).toUpperCase()}
                </span>
                <span className="sidebar-ws-option-copy">
                  <strong>{ws.name}</strong>
                  {wsOrg && organizations.length > 1 ? (
                    <span>{wsOrg.name}</span>
                  ) : null}
                </span>
                {active ? (
                  <svg className="sidebar-ws-check" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                    <path d="M3.5 8.5l3 3 6-7" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                ) : null}
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
