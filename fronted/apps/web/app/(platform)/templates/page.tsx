import { PageHeader } from "@/components/ui/PageHeader";
import { BffResourcePanel } from "@/components/platform/BffResourcePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function TemplatesPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Template"
        title="模板管理"
        description="项目模板 CRUD（对应旧 /template/list）"
      />
      <BffResourcePanel
        workspaceId={workspaceId}
        title="模板"
        bffPath="/workspaces/{wsId}/templates"
        upstreamPath="/api/v1/workspaces/:wsId/templates"
      />
    </div>
  );
}
