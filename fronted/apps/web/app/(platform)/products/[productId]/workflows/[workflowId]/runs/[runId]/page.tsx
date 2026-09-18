import { ProductWorkflowRunDetailPanel } from "@/modules/product/components/ProductWorkflowRunDetailPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProductWorkflowRunPage({
  params,
}: {
  params: Promise<{ productId: string; workflowId: string; runId: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const { productId, workflowId, runId } = await params;
  return (
    <ProductWorkflowRunDetailPanel
      workspaceId={workspaceId}
      productId={productId}
      workflowId={workflowId}
      runId={runId}
    />
  );
}
