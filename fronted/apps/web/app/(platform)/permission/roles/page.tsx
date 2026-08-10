import { RoleTreePanel } from "@/modules/governance/components/RoleTreePanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function PermissionRolesPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法管理角色。请联系 Workspace 管理员。
      </div>
    );
  }

  return (
    <section className="section stack-md">
      <RoleTreePanel workspaceId={workspaceId} />
    </section>
  );
}
