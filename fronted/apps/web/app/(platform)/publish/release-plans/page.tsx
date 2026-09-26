import { PageHeader } from "@/components/ui/PageHeader";
import { WorkspaceReleasePlansPanel } from "@/modules/product/components/WorkspaceReleasePlansPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishReleasePlansPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Publish"
        title="发布计划"
        description="跨产品查看发布计划，支持模板复用、列表与日历视图"
      />
      <WorkspaceReleasePlansPanel workspaceId={workspaceId} />
    </div>
  );
}
