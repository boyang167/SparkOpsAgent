import { Suspense } from "react";
import { ProductWorkflowDetailPanel } from "@/modules/product/components/ProductWorkflowDetailPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

function ProductWorkflowDetailContent({
  workspaceId,
  productId,
  workflowId,
}: {
  workspaceId: string;
  productId: string;
  workflowId: string;
}) {
  return (
    <ProductWorkflowDetailPanel workspaceId={workspaceId} productId={productId} workflowId={workflowId} />
  );
}

export default async function ProductWorkflowDetailPage({
  params,
}: {
  params: Promise<{ productId: string; workflowId: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const { productId, workflowId } = await params;
  return (
    <Suspense fallback={<div className="empty-state text-sm">加载工作流…</div>}>
      <ProductWorkflowDetailContent workspaceId={workspaceId} productId={productId} workflowId={workflowId} />
    </Suspense>
  );
}
