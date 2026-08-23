import { ClusterSyncPanel } from "@/modules/platform_admin/components/ClusterSyncPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PlatformClusterSyncPage() {
  await requirePlatformConsoleAccess();
  const { workspaceId } = await loadPlatformPageContext();
  return (
    <div className="platform-page">
      <ClusterSyncPanel workspaceId={workspaceId} variant="platform" />
    </div>
  );
}
