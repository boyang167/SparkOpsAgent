import { PageHeader } from "@/components/ui/PageHeader";
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
    <div className="stack-lg">
      <PageHeader
        eyebrow="DevOps"
        title="项目管理"
        description="管理产品下的项目，创建、配置并进入项目工作区"
      />
      <ProjectPanel workspaceId={workspaceId} initialCreateOpen={params.create === "1"} />
    </div>
  );
}
