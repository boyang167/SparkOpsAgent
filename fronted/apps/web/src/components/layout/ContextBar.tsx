"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { isOrgReadOnly, isWorkspaceReadOnly } from "@/modules/tenant/lib/org-display";

interface ProjectOption {
  id: string;
  name: string;
}

interface ContextBarProps {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
  currentWorkspaceId?: string;
  projects?: ProjectOption[];
  currentProjectId?: string;
}

export function ContextBar({
  organizations,
  workspaces,
  currentWorkspaceId,
  projects = [],
  currentProjectId,
}: ContextBarProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);

  const switchableWorkspaces = useMemo(
    () => workspaces.filter((ws) => !isWorkspaceReadOnly(ws)),
    [workspaces],
  );

  const currentWorkspace =
    switchableWorkspaces.find((ws) => ws.id === currentWorkspaceId) ??
    switchableWorkspaces[0] ??
    workspaces.find((ws) => ws.id === currentWorkspaceId) ??
    workspaces[0];

  const currentOrgId = currentWorkspace?.org_id;

  const orgWorkspaces = useMemo(
    () => (currentOrgId ? workspaces.filter((ws) => ws.org_id === currentOrgId) : workspaces),
    [currentOrgId, workspaces],
  );

  async function switchWorkspace(workspaceId: string) {
    const target = workspaces.find((ws) => ws.id === workspaceId);
    if (!target || isWorkspaceReadOnly(target)) return;
    if (workspaceId === currentWorkspaceId || loading) return;
    setLoading(true);
    try {
      await tenant.selectWorkspace(workspaceId);
      router.refresh();
    } finally {
      setLoading(false);
    }
  }

  async function onOrgChange(orgId: string) {
    const org = organizations.find((o) => o.id === orgId);
    if (org && isOrgReadOnly(org)) return;
    const nextWorkspace = workspaces.find(
      (ws) => ws.org_id === orgId && !isWorkspaceReadOnly(ws),
    );
    if (nextWorkspace) {
      await switchWorkspace(nextWorkspace.id);
    }
  }

  function onProjectChange(projectId: string) {
    if (projectId === currentProjectId) return;
    router.push(`/projects/${projectId}`);
  }

  if (workspaces.length === 0 && organizations.length === 0) {
    return null;
  }

  const showOrgSelect = organizations.length > 1;

  return (
    <div className="context-bar">
      {showOrgSelect ? (
        <>
          <label className="context-bar-field">
            <span className="context-bar-label">组织</span>
            <select
              className="context-bar-select"
              value={currentOrgId ?? ""}
              disabled={loading}
              onChange={(e) => void onOrgChange(e.target.value)}
            >
              {organizations.map((org) => (
                <option key={org.id} value={org.id} disabled={isOrgReadOnly(org)}>
                  {org.name}
                  {isOrgReadOnly(org) ? "（仅查看）" : ""}
                </option>
              ))}
            </select>
          </label>
          <span className="context-bar-sep" aria-hidden>
            /
          </span>
        </>
      ) : currentOrganizationLabel(organizations, currentOrgId) ? (
        <>
          <span className="context-bar-static">
            {currentOrganizationLabel(organizations, currentOrgId)}
            {organizations[0] && isOrgReadOnly(organizations[0]) ? "（仅查看）" : ""}
          </span>
          <span className="context-bar-sep" aria-hidden>
            /
          </span>
        </>
      ) : null}

      <label className="context-bar-field">
        <span className="context-bar-label">工作区</span>
        <select
          className="context-bar-select"
          value={currentWorkspaceId ?? switchableWorkspaces[0]?.id ?? workspaces[0]?.id ?? ""}
          disabled={loading || switchableWorkspaces.length === 0}
          onChange={(e) => void switchWorkspace(e.target.value)}
        >
          {(organizations.length > 1 ? orgWorkspaces : workspaces).map((ws) => (
            <option key={ws.id} value={ws.id} disabled={isWorkspaceReadOnly(ws)}>
              {ws.name}
              {isWorkspaceReadOnly(ws) ? "（仅查看）" : ""}
            </option>
          ))}
        </select>
      </label>

      {currentProjectId && projects.length > 0 ? (
        <>
          <span className="context-bar-sep" aria-hidden>
            /
          </span>
          <label className="context-bar-field">
            <span className="context-bar-label">项目</span>
            <select
              className="context-bar-select"
              value={currentProjectId}
              onChange={(e) => onProjectChange(e.target.value)}
            >
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>
        </>
      ) : null}
    </div>
  );
}

function currentOrganizationLabel(
  organizations: OrganizationVM[],
  orgId?: string,
): string | undefined {
  if (!orgId) return undefined;
  return organizations.find((org) => org.id === orgId)?.name;
}
