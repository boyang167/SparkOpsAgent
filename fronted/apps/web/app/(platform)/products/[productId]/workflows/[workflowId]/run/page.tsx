import { redirect } from "next/navigation";

export default async function ProductWorkflowRunStartPage({
  params,
}: {
  params: Promise<{ productId: string; workflowId: string }>;
}) {
  const { productId, workflowId } = await params;
  redirect(`/products/${productId}/workflows/${workflowId}?run=1`);
}
