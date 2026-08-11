import { OperationRecordsPanel } from "@/modules/platform_admin/components/OperationRecordsPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function MonitorOperationPage() {
  const { workspaceId, canManage, members } = await loadGovernanceContext();

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看操作日志。请联系 Workspace 管理员。
      </div>
    );
  }

  return <OperationRecordsPanel workspaceId={workspaceId} members={members} />;
}
