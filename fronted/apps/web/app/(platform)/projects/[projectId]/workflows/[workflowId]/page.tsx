import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { WorkflowDetailPanel } from "@/modules/workflow/components/WorkflowDetailPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { WorkflowRunPanel } from "@/modules/workflow/components/WorkflowRunPanel";
import { loadWorkflow, loadWorkflowRuns } from "@/server/loaders/workflow";

interface WorkflowDetailPageProps {
  params: Promise<{ projectId: string; workflowId: string }>;
}

export default async function WorkflowDetailPage({ params }: WorkflowDetailPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { projectId, workflowId } = await params;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let wf;
  try {
    wf = await loadWorkflow(workspaceId, workflowId);
  } catch {
    notFound();
  }

  const runs = await loadWorkflowRuns(workspaceId, workflowId);

  return (
    <div className="stack-lg">
      <Link href={`/projects/${projectId}`} className="link-back">
        ← 返回项目
      </Link>

      <PageHeader eyebrow="Workflow" title={wf.name} description={wf.description || undefined} />

      <WorkflowDetailPanel workspaceId={workspaceId} projectId={projectId} workflow={wf} />

      <WorkflowRunPanel
        projectId={projectId}
        workspaceId={workspaceId}
        workflowId={workflowId}
        workflowStatus={wf.status}
        runs={runs}
      />
    </div>
  );
}
