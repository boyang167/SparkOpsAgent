import { ProductDetailPanel } from "@/modules/product/components/ProductDetailPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

interface ProductDetailPageProps {
  params: Promise<{ productId: string }>;
}

export default async function ProductDetailPage({ params }: ProductDetailPageProps) {
  const { productId } = await params;
  const { workspaceId } = await loadPlatformPageContext();

  return <ProductDetailPanel workspaceId={workspaceId} productId={productId} />;
}
