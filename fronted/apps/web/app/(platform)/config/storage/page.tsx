import { PageHeader } from "@/components/ui/PageHeader";
import { K8sResourceTablePanel } from "@/modules/infra/components/K8sResourceTablePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function StoragePage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Config" title="持久化存储" description="PVC 列表（对应旧 /conf/pvc/list）" />
      <K8sResourceTablePanel
        workspaceId={workspaceId}
        projects={projects}
        kind="pvcs"
        title="PVC"
        initialProjectId={projects[0]?.id}
      />
    </div>
  );
}
