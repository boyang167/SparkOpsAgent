import { InviteMemberForm } from "@/modules/governance/components/InviteMemberForm";
import { MemberBindings } from "@/modules/governance/components/MemberBindings";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function PermissionMembersPage() {
  const { workspaceId, canManage, roles, members } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法管理成员绑定。请联系 Workspace 管理员。
      </div>
    );
  }

  return (
    <section className="section stack-md">
      <div>
        <h2 className="section-title">成员绑定</h2>
        <p className="text-muted mt-1">邀请已注册用户加入 Workspace，并分配 RBAC 角色</p>
      </div>
      <InviteMemberForm workspaceId={workspaceId} />
      <MemberBindings workspaceId={workspaceId} members={members} roles={roles} />
    </section>
  );
}
