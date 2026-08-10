import { PermissionApiTokensPanel } from "@/modules/governance/components/PermissionApiTokensPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";
import { loadBffContext } from "@/server/loaders/context";
import { listMembers, listRoleTree } from "@sparkops/bff-modules/governance";

export default async function PermissionApiTokensPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法管理 API Token。
      </div>
    );
  }

  const ctx = await loadBffContext(workspaceId);
  const [roles, members] = await Promise.all([
    listRoleTree(ctx, workspaceId),
    listMembers(ctx, workspaceId),
  ]);

  return <PermissionApiTokensPanel workspaceId={workspaceId} members={members} roles={roles} />;
}
