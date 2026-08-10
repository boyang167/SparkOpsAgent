"use client";

import { useState } from "react";
import Link from "next/link";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { CreateOrganizationDialog } from "./CreateOrganizationDialog";
import { CreateWorkspaceDialog } from "./CreateWorkspaceDialog";

interface TenantSetupPanelProps {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
}

export function TenantSetupPanel({ organizations, workspaces }: TenantSetupPanelProps) {
  const [orgDialogOpen, setOrgDialogOpen] = useState(organizations.length === 0);
  const [wsDialogOpen, setWsDialogOpen] = useState(false);
  const [wsOrgId, setWsOrgId] = useState<string | undefined>();

  const orgOptions = organizations.map((org) => ({
    id: org.id,
    name: org.name,
    slug: org.slug,
  }));

  return (
    <div className="tenant-setup stack-md">
      <CreateOrganizationDialog
        open={orgDialogOpen}
        onClose={() => setOrgDialogOpen(false)}
        onContinueToWorkspace={(org) => {
          setOrgDialogOpen(false);
          setWsOrgId(org.id);
          setWsDialogOpen(true);
        }}
      />
      <CreateWorkspaceDialog
        open={wsDialogOpen}
        onClose={() => setWsDialogOpen(false)}
        organizations={orgOptions}
        defaultOrgId={wsOrgId}
      />

      <div className="tenant-setup-card">
        <h3 className="section-title">欢迎使用 SparkOps</h3>
        <p className="text-muted mt-2">
          完成租户初始化后即可创建项目与 Agent。建议按「组织 → 工作区」顺序配置。
        </p>
        <ol className="tenant-setup-steps">
          <li>
            <span className="tenant-setup-step-num">1</span>
            <div>
              <p className="font-medium">创建组织</p>
              <p className="text-muted text-sm mt-0.5">企业或团队的顶层容器</p>
            </div>
            <button type="button" className="btn-secondary text-sm" onClick={() => setOrgDialogOpen(true)}>
              开始
            </button>
          </li>
          <li>
            <span className="tenant-setup-step-num">2</span>
            <div>
              <p className="font-medium">创建工作区</p>
              <p className="text-muted text-sm mt-0.5">日常协作与资源边界</p>
            </div>
            <button
              type="button"
              className="btn text-sm"
              disabled={organizations.length === 0}
              onClick={() => setWsDialogOpen(true)}
            >
              创建
            </button>
          </li>
        </ol>
        <p className="text-sm text-muted mt-4">
          也可前往{" "}
          <Link href="/settings/tenant" className="link-primary">
            组织与工作区
          </Link>{" "}
          统一管理。
        </p>
      </div>
    </div>
  );
}
