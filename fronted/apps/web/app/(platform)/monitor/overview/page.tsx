import { PageHeader } from "@/components/ui/PageHeader";
import { MonitorOverviewPanel } from "@/modules/platform_admin/components/MonitorOverviewPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function MonitorOverviewPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Monitor" title="监控概览" description="Grafana / Prometheus 外链与面板配置" />
      <MonitorOverviewPanel workspaceId={workspaceId} />
    </div>
  );
}
