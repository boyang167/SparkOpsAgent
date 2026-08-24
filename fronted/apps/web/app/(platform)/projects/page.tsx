import { ProjectPanel } from "@/modules/workspace/components/ProjectPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ProjectsPage({
  searchParams,
}: {
  searchParams: Promise<{ create?: string }>;
}) {
  const { workspaceId } = await loadPlatformPageContext();
  const params = await searchParams;

  return (
    <ProjectPanel workspaceId={workspaceId} initialCreateOpen={params.create === "1"} />
  );
}
