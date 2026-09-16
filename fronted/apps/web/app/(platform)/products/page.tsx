import { ProductsPanel } from "@/modules/product/components/ProductsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProductsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <ProductsPanel
      workspaceId={workspaceId}
      title="产品管理"
      description="管理工作空间下的产品及其关联项目、环境与发布计划。"
      linkToDetail
    />
  );
}
