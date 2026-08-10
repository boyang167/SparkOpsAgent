import { PageHeader } from "@/components/ui/PageHeader";
import { DeploymentStatsPanel } from "@/modules/platform_admin/components/DeploymentStatsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function MonitorStatisticsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Monitor" title="部署统计" description="工作区部署/项目统计" />
      <DeploymentStatsPanel workspaceId={workspaceId} />
    </div>
  );
}
