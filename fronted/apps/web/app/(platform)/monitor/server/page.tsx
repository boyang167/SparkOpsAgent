import { ServerStatusPanel } from "@/modules/platform_admin/components/ServerStatusPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function MonitorServerPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看服务器状态。请联系 Workspace 管理员。
      </div>
    );
  }

  return <ServerStatusPanel workspaceId={workspaceId} />;
}
