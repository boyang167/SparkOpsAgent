import { PageHeader } from "@/components/ui/PageHeader";
import { BffResourcePanel } from "@/components/platform/BffResourcePanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function DockerfilesMarketPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Market"
        title="Dockerfile 市场"
        description="共享 Dockerfile 模板（对应旧 /markets/dockerfile/list）"
      />
      <BffResourcePanel
        workspaceId={workspaceId}
        title="Dockerfile"
        bffPath="/workspaces/{wsId}/market/dockerfiles"
        upstreamPath="/api/v1/workspaces/:wsId/market/dockerfiles"
      />
    </div>
  );
}
