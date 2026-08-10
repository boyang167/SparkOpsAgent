import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { DeliveryBuildPanel } from "@/modules/delivery/components/DeliveryBuildPanel";
import { loadBuilds } from "@/server/loaders/delivery";
import { loadProjects } from "@/server/loaders/projects";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";

export default async function DeliveryBuildsPage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ?? workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  const projects = await loadProjects(workspaceId);
  const firstProjectId = projects[0]?.id;
  const builds = firstProjectId ? await loadBuilds(workspaceId, firstProjectId) : [];

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Delivery"
        title="构建中心"
        description="对应 PlatformSuite Jenkins 构建能力（当前为 Mock 执行器）"
      />
      <DeliveryBuildPanel
        workspaceId={workspaceId}
        projects={projects}
        initialBuilds={builds}
        initialProjectId={firstProjectId}
      />
    </div>
  );
}
