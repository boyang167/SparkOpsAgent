import { ProductWorkflowsHubPanel } from "@/modules/product/components/ProductWorkflowsHubPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProductWorkflowsPage() {
  const { workspaceId } = await loadPlatformPageContext();
  return <ProductWorkflowsHubPanel workspaceId={workspaceId} />;
}
