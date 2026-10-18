import { DeploymentStatsPanel } from "@/modules/platform_admin/components/DeploymentStatsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function MonitorStatisticsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return <DeploymentStatsPanel workspaceId={workspaceId} />;
}
