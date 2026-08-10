import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { WorkflowRunDetailPanel } from "@/modules/workflow/components/WorkflowRunDetailPanel";
import { formatWorkflowTime, statusBadgeClass } from "@/modules/workflow/utils/workflowRun";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadWorkflow, loadWorkflowRun } from "@/server/loaders/workflow";

interface WorkflowRunDetailPageProps {
  params: Promise<{ projectId: string; workflowId: string; runId: string }>;
}

export default async function WorkflowRunDetailPage({ params }: WorkflowRunDetailPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { projectId, workflowId, runId } = await params;
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
  let run;
  try {
    [wf, run] = await Promise.all([
      loadWorkflow(workspaceId, workflowId),
      loadWorkflowRun(workspaceId, runId),
    ]);
  } catch {
    notFound();
  }

  if (run.workflow_id !== workflowId) {
    notFound();
  }

  return (
    <div className="stack-lg">
      <Link href={`/projects/${projectId}/workflows/${workflowId}`} className="link-back">
        ← 返回 Workflow
      </Link>

      <PageHeader
        eyebrow={wf.name}
        title="运行详情"
        description={formatWorkflowTime(run.started_at)}
        actions={<span className={`badge ${statusBadgeClass(run.status)}`}>{run.status}</span>}
      />

      <WorkflowRunDetailPanel run={run} />
    </div>
  );
}
