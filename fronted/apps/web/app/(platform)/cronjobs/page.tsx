import { PageHeader } from "@/components/ui/PageHeader";
import { CronJobsPanel } from "@/modules/delivery/components/CronJobsPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function CronJobsPage() {
  const { workspaceId, projects } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Delivery" title="定时任务" description="Cron 触发构建（对应旧 /cornjob/list）" />
      <CronJobsPanel workspaceId={workspaceId} projects={projects} initialProjectId={projects[0]?.id} />
    </div>
  );
}
