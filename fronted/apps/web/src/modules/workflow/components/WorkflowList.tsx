import Link from "next/link";
import type { WorkflowVM } from "@sparkops/contracts/workflow";

interface WorkflowListProps {
  workflows: WorkflowVM[];
  projectId: string;
}

export function WorkflowList({ workflows, projectId }: WorkflowListProps) {
  if (workflows.length === 0) {
    return <div className="empty-state">该项目下暂无 Workflow，点击「新建 Workflow」创建。</div>;
  }

  return (
    <div className="grid-cards">
      {workflows.map((workflow) => (
        <Link
          key={workflow.id}
          href={`/projects/${projectId}/workflows/${workflow.id}`}
          className="card card-hover card-link"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="font-semibold truncate">{workflow.name}</p>
              <p className="text-muted mt-1.5 line-clamp-2">
                {workflow.description || "暂无描述"}
              </p>
            </div>
            <span className="badge shrink-0 badge-neutral">{workflow.status}</span>
          </div>
        </Link>
      ))}
    </div>
  );
}
