import { ProductWorkflowEditorPage } from "@/modules/product/components/ProductWorkflowEditorPage";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function EditProductWorkflowPage({
  params,
}: {
  params: Promise<{ productId: string; workflowId: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const { productId, workflowId } = await params;
  return (
    <ProductWorkflowEditorPage workspaceId={workspaceId} productId={productId} workflowId={workflowId} />
  );
}
