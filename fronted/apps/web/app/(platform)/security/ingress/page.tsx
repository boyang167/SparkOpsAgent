import { PageHeader } from "@/components/ui/PageHeader";
import { K8sResourceTablePanel } from "@/modules/infra/components/K8sResourceTablePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function IngressPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Security" title="Ingress" description="入口/API 列表（对应旧 /security/ingress/list）" />
      <K8sResourceTablePanel
        workspaceId={workspaceId}
        projects={projects}
        kind="ingresses"
        title="Ingress"
        initialProjectId={projects[0]?.id}
      />
    </div>
  );
}
