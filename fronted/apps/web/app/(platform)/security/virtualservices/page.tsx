import { PageHeader } from "@/components/ui/PageHeader";
import { K8sResourceTablePanel } from "@/modules/infra/components/K8sResourceTablePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function VirtualServicesPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Security"
        title="VirtualService"
        description="Istio 路由规则（当前为 Mock 数据，待接入 istio client-go）"
      />
      <K8sResourceTablePanel
        workspaceId={workspaceId}
        projects={projects}
        kind="virtualservices"
        title="VirtualService"
        initialProjectId={projects[0]?.id}
      />
    </div>
  );
}
