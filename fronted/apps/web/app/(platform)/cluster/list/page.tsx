import { PageHeader } from "@/components/ui/PageHeader";
import { WorkspaceClusterPanel } from "@/modules/infra/components/WorkspaceClusterPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ClusterListPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="基础设施"
        title="集群管理"
        description="为当前工作空间注册 Kubernetes 集群。注册后即可绑定到项目并用于部署。"
      />
      <WorkspaceClusterPanel workspaceId={workspaceId} />
    </div>
  );
}
