import { ClusterListPanel } from "@/modules/platform_admin/components/ClusterListPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformClustersPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <ClusterListPanel variant="platform" />
    </div>
  );
}
