"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";

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

  const currentWorkspace = workspaces.find((ws) => ws.id === currentWorkspaceId) ?? workspaces[0];
  const currentOrgId = currentWorkspace?.org_id;

  const orgWorkspaces = useMemo(
    () => (currentOrgId ? workspaces.filter((ws) => ws.org_id === currentOrgId) : workspaces),
    [currentOrgId, workspaces],
  );

  async function switchWorkspace(workspaceId: string) {
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
    const nextWorkspace = workspaces.find((ws) => ws.org_id === orgId);
    if (nextWorkspace) {
      await switchWorkspace(nextWorkspace.id);
    }
  }

  function onProjectChange(projectId: string) {
    if (projectId === currentProjectId) return;
    router.push(`/projects/${projectId}`);
  }

  if (workspaces.length === 0) {
    return null;
  }

  return (
    <div className="context-bar">
      {organizations.length > 1 ? (
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
                <option key={org.id} value={org.id}>
                  {org.name}
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
          <span className="context-bar-static">{currentOrganizationLabel(organizations, currentOrgId)}</span>
          <span className="context-bar-sep" aria-hidden>
            /
          </span>
        </>
      ) : null}

      <label className="context-bar-field">
        <span className="context-bar-label">工作区</span>
        <select
          className="context-bar-select"
          value={currentWorkspaceId ?? workspaces[0]?.id ?? ""}
          disabled={loading}
          onChange={(e) => void switchWorkspace(e.target.value)}
        >
          {(organizations.length > 1 ? orgWorkspaces : workspaces).map((ws) => (
            <option key={ws.id} value={ws.id}>
              {ws.name}
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
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </label>
        </>
      ) : null}
    </div>
  );
}

function currentOrganizationLabel(organizations: OrganizationVM[], orgId?: string) {
  if (!orgId) return null;
  return organizations.find((org) => org.id === orgId)?.name ?? null;
}
