"use client";

import { useState } from "react";
import Link from "next/link";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { CreateOrganizationDialog } from "./CreateOrganizationDialog";
import { CreateWorkspaceDialog } from "./CreateWorkspaceDialog";

interface CreateWorkspaceFormProps {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
}

export function CreateWorkspaceForm({ organizations, workspaces }: CreateWorkspaceFormProps) {
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [wsDialogOpen, setWsDialogOpen] = useState(false);

  return (
    <>
      <CreateOrganizationDialog
        open={orgDialogOpen}
        onClose={() => setOrgDialogOpen(false)}
        onContinueToWorkspace={() => {
          setOrgDialogOpen(false);
          setWsDialogOpen(true);
        }}
      />
      <CreateWorkspaceDialog
        open={wsDialogOpen}
        onClose={() => setWsDialogOpen(false)}
        organizations={organizations.map((org) => ({
          id: org.id,
          name: org.name,
          slug: org.slug,
        }))}
      />

      <div className="flex flex-wrap items-center gap-2">
        <button className="btn-secondary" type="button" onClick={() => setWsDialogOpen(true)}>
          新建工作区
        </button>
        <button className="btn-ghost text-sm" type="button" onClick={() => setOrgDialogOpen(true)}>
          新建组织
        </button>
        <Link href="/settings/tenant" className="btn-ghost text-sm">
          组织管理
        </Link>
      </div>
    </>
  );
}
