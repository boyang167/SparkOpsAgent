import { PermissionMenusPanel } from "@/modules/governance/components/PermissionMenusPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function PermissionMenusPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看菜单管理。
      </div>
    );
  }

  return (
    <section className="section !p-0 !border-0 !shadow-none !bg-transparent stack-lg">
      <PermissionMenusPanel workspaceId={workspaceId} />
    </section>
  );
}
