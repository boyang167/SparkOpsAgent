import { PageHeader } from "@/components/ui/PageHeader";
import { K8sResourceTablePanel } from "@/modules/infra/components/K8sResourceTablePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function DiscoveryServicesPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Discovery" title="服务发现" description="K8s Service 列表（对应旧 /discovery/services/list）" />
      <K8sResourceTablePanel
        workspaceId={workspaceId}
        projects={projects}
        kind="services"
        title="Service"
        initialProjectId={projects[0]?.id}
      />
    </div>
  );
}
