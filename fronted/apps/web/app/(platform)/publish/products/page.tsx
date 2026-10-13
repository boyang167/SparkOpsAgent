import { ProductsPanel } from "@/modules/product/components/ProductsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishProductsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <ProductsPanel
      workspaceId={workspaceId}
      title="产品清单"
      description="按产品统一管理项目、环境与发布计划。"
      linkToDetail
    />
  );
}
