import { PageHeader } from "@/components/ui/PageHeader";
import { PublishReleasesPanel } from "@/modules/product/components/PublishReleasesPanel";
import { loadBffContext } from "@/server/loaders/context";
import { listProducts } from "@sparkops/bff-modules/product";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishReleasesPage() {
  const { workspaceId } = await loadPlatformPageContext();
  const ctx = await loadBffContext(workspaceId);
  const products = await listProducts(ctx, workspaceId);

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Publish"
        title="应用发布"
        description="发布需求审批、发布与跨集群同步（对应旧 /AppRelease requirement 流程）"
      />
      <PublishReleasesPanel workspaceId={workspaceId} products={products} />
    </div>
  );
}
