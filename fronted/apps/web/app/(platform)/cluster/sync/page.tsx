import { PageHeader } from "@/components/ui/PageHeader";
import { ClusterSyncPanel } from "@/modules/platform_admin/components/ClusterSyncPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ClusterSyncPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Cluster" title="资源同步" description="跨集群资源同步任务（对应旧 /cluster/ResourceSync）" />
      <ClusterSyncPanel workspaceId={workspaceId} />
    </div>
  );
}
