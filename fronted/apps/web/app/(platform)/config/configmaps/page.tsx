import { PageHeader } from "@/components/ui/PageHeader";
import { K8sResourceTablePanel } from "@/modules/infra/components/K8sResourceTablePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ConfigMapsPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Config" title="配置字典" description="K8s ConfigMap 列表（对应旧 /conf/configMap）" />
      <K8sResourceTablePanel
        workspaceId={workspaceId}
        projects={projects}
        kind="configmaps"
        title="ConfigMap"
        initialProjectId={projects[0]?.id}
      />
    </div>
  );
}
