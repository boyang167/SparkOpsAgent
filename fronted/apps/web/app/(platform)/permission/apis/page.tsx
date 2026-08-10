import { PermissionApisPanel } from "@/modules/governance/components/PermissionApisPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";
import { loadSession } from "@/server/loaders/session";

export default async function PermissionApisPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();
  const session = await loadSession();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看 API 管理。
      </div>
    );
  }

  return <PermissionApisPanel workspaceId={workspaceId} canSync={session?.is_super_admin ?? false} />;
}
