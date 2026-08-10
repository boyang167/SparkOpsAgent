import { PageHeader } from "@/components/ui/PageHeader";
import { SysConfigPanel } from "@/modules/platform_admin/components/SysConfigPanel";

export default function ClusterConfigPage() {
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Cluster" title="密钥配置" description="Grafana / Prometheus / Jenkins 等系统级配置" />
      <SysConfigPanel />
    </div>
  );
}
