import { PageHeader } from "@/components/ui/PageHeader";
import { ClusterListPanel } from "@/modules/platform_admin/components/ClusterListPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformClustersPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="平台运营" title="集群管理" description="多集群接入与 Kubeconfig 注册。" />
      <ClusterListPanel />
    </div>
  );
}
