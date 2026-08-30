import { WorkspacePluginsPanel } from "@/modules/plugin/components/WorkspacePluginsPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function WorkspacePluginsPage() {
  const { workspaceId, canManage } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法管理工作区插件。
      </div>
    );
  }

  return <WorkspacePluginsPanel workspaceId={workspaceId} />;
}
