import { LogViewerPanel } from "@/modules/platform_admin/components/LogViewerPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function MonitorLogViewerPage({
  searchParams,
}: {
  searchParams: Promise<{ trace?: string }>;
}) {
  const { workspaceId, canManage } = await loadGovernanceContext();
  const query = await searchParams;

  if (!canManage) {
    return (
      <div className="alert alert-warning">
        你没有 governance:admin 权限，无法查看日志。请联系 Workspace 管理员。
      </div>
    );
  }

  return <LogViewerPanel workspaceId={workspaceId} initialTraceId={query.trace} />;
}
