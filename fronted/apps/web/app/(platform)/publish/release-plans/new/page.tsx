import { ReleasePlanCreatePage } from "@/modules/product/components/ReleasePlanCreatePage";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ReleasePlanCreateRoute({
  searchParams,
}: {
  searchParams: Promise<{ productId?: string; sprintId?: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const { productId, sprintId } = await searchParams;
  return (
    <ReleasePlanCreatePage
      workspaceId={workspaceId}
      defaultProductId={productId}
      defaultSprintId={sprintId}
    />
  );
}
