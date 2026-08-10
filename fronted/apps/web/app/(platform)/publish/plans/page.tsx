import { PageHeader } from "@/components/ui/PageHeader";
import { PublishPlansPanel } from "@/modules/product/components/PublishPlansPanel";
import { loadBffContext } from "@/server/loaders/context";
import { listProducts } from "@sparkops/bff-modules/product";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishPlansPage() {
  const { workspaceId } = await loadPlatformPageContext();
  const ctx = await loadBffContext(workspaceId);
  const products = await listProducts(ctx, workspaceId);

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Publish" title="迭代计划" description="产品迭代与版本计划（对应旧 /IterationPlan）" />
      <PublishPlansPanel workspaceId={workspaceId} products={products} />
    </div>
  );
}
