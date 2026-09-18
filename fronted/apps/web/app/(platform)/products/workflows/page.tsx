import { ProductWorkflowsHubPanel } from "@/modules/product/components/ProductWorkflowsHubPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProductWorkflowsPage({
  searchParams,
}: {
  searchParams: Promise<{ productId?: string; workflowId?: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const params = await searchParams;
  return (
    <ProductWorkflowsHubPanel
      workspaceId={workspaceId}
      initialProductId={params.productId ?? null}
      initialWorkflowId={params.workflowId ?? null}
    />
  );
}
