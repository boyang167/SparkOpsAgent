import { PageHeader } from "@/components/ui/PageHeader";
import { BffResourcePanel } from "@/components/platform/BffResourcePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function WorkplacePage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Dashboard"
        title="工作台"
        description="聚合项目、构建与运行态概览（对应旧 /workplace）"
      />
      <BffResourcePanel
        workspaceId={workspaceId}
        title="工作台概览"
        bffPath="/workspaces/{wsId}/dashboard"
        upstreamPath="/api/v1/workspaces/:wsId/dashboard"
      />
    </div>
  );
}
