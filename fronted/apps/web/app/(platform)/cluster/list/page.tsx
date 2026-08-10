import { PageHeader } from "@/components/ui/PageHeader";
import { ClusterListPanel } from "@/modules/platform_admin/components/ClusterListPanel";

export default function ClusterListPage() {
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Cluster" title="集群列表" description="多集群 Kubeconfig 注册（对应旧 /cluster/list）" />
      <ClusterListPanel />
    </div>
  );
}
