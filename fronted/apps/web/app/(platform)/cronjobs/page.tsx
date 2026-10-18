import { PageHeader } from "@/components/ui/PageHeader";
import { CronJobsPanel } from "@/modules/delivery/components/CronJobsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function CronJobsPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Delivery" title="定时构建" description="按计划自动触发项目构建任务" />
      <CronJobsPanel workspaceId={workspaceId} projects={projects} initialProjectId={projects[0]?.id} />
    </div>
  );
}
