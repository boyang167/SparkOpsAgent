import { TemplatesHub } from "@/modules/workspace/components/TemplatesHub";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function TemplatesPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return <TemplatesHub workspaceId={workspaceId} />;
}
