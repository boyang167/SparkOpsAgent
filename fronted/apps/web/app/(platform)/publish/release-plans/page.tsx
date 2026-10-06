import { PublishReleasePlansView } from "@/modules/product/components/PublishReleasePlansView";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function PublishReleasePlansPage() {
  const { workspaceId } = await loadPlatformPageContext();
  return <PublishReleasePlansView workspaceId={workspaceId} />;
}
