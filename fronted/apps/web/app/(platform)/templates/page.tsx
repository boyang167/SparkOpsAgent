import { PageHeader } from "@/components/ui/PageHeader";
import { TemplatesHub } from "@/modules/workspace/components/TemplatesHub";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function TemplatesPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Template"
        title="模板管理"
        description="管理项目脚手架与工作空间级迭代流程模板"
      />
      <TemplatesHub workspaceId={workspaceId} />
    </div>
  );
}
