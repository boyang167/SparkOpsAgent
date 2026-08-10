import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { RuntimePodPanel } from "@/modules/runtime/components/RuntimePodPanel";
import { loadPods } from "@/server/loaders/delivery";
import { loadProjects } from "@/server/loaders/projects";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";

export default async function RuntimePodsPage() {
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
  const pods = firstProjectId ? await loadPods(workspaceId, firstProjectId) : [];

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Runtime"
        title="K8s Pod"
        description="对应 PlatformSuite Pod 运维视图（当前为 Mock 集群）"
      />
      <RuntimePodPanel
        workspaceId={workspaceId}
        projects={projects}
        initialPods={pods}
        initialProjectId={firstProjectId}
      />
    </div>
  );
}
