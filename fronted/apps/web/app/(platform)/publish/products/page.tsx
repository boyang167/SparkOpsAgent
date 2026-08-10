import { PageHeader } from "@/components/ui/PageHeader";
import { ProductsPanel } from "@/modules/product/components/ProductsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProductsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Publish" title="产品清单" description="发布中心产品列表（对应旧 /PublishCenter/ProductList）" />
      <ProductsPanel workspaceId={workspaceId} />
    </div>
  );
}
