import { PageHeader } from "@/components/ui/PageHeader";
import { TemplatePanel } from "@/modules/workspace/components/TemplatePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function TemplatesPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Template"
        title="模板管理"
        description="管理和维护项目模板"
      />
      <TemplatePanel workspaceId={workspaceId} />
    </div>
  );
}
