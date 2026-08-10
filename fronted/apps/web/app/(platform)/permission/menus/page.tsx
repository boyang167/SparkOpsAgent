import { PermissionMenusPanel } from "@/modules/governance/components/PermissionMenusPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function PermissionMenusPage() {
  const { canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看菜单管理。
      </div>
    );
  }

  return <PermissionMenusPanel />;
}
