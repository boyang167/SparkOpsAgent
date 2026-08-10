import { PageHeader } from "@/components/ui/PageHeader";
import { ServerStatusPanel } from "@/modules/platform_admin/components/ServerStatusPanel";

export default function MonitorServerPage() {
  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Monitor"
        title="服务器状态"
        description="API 进程运行时指标（参照 GVA / PlatformSuite 监控）"
      />
      <ServerStatusPanel />
    </div>
  );
}
