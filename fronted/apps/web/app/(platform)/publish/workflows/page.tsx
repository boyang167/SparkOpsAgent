import { PageHeader } from "@/components/ui/PageHeader";
import { WorkspaceWorkflowsPanel } from "@/modules/product/components/WorkspaceWorkflowsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishWorkflowsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Publish"
        title="产品工作流"
        description="跨产品查看与一键执行 ProductWorkflow（对标 releaseWorkflow 全局视图）"
      />
      <WorkspaceWorkflowsPanel workspaceId={workspaceId} />
    </div>
  );
}
