"use client";

import type { RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InviteMemberForm } from "@/modules/governance/components/InviteMemberForm";
import { MemberBindings } from "@/modules/governance/components/MemberBindings";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";

interface AdminMembersPanelProps {
  workspaceId: string;
  members: WorkspaceMemberVM[];
  roles: RoleVM[];
}

export function AdminMembersPanel({ workspaceId, members, roles }: AdminMembersPanelProps) {
  return (
    <div className="admin-members-page">
      <section className="platform-table-card admin-members-card">
        <PlatformPanelHead
          title="成员管理"
          description="邀请成员加入当前工作区，并分配 RBAC 角色。"
        />
        <div className="admin-members-toolbar">
          <InviteMemberForm workspaceId={workspaceId} variant="admin" />
          <span className="admin-members-count">{members.length} 位成员</span>
        </div>
        <div className="admin-members-body">
          <MemberBindings
            workspaceId={workspaceId}
            members={members}
            roles={roles}
            variant="admin"
          />
        </div>
      </section>
    </div>
  );
}
