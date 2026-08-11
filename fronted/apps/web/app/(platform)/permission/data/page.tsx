import { PermissionDataPanel } from "@/modules/governance/components/PermissionDataPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function PermissionDataPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看数据权限管理。
      </div>
    );
  }

  return <PermissionDataPanel workspaceId={workspaceId} />;
}
