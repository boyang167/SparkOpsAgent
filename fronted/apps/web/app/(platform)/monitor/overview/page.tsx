import { MonitorOverviewPanel } from "@/modules/platform_admin/components/MonitorOverviewPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function MonitorOverviewPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return <MonitorOverviewPanel workspaceId={workspaceId} />;
}
