import Link from "next/link";

import type { AgentVM } from "@sparkops/contracts/agent";



interface AgentListProps {

  agents: AgentVM[];

  projectId: string;

}



function statusBadge(status: string) {

  if (status === "published") {

    return "badge-success";

  }

  if (status === "draft") {

    return "badge-warning";

  }

  return "badge-neutral";

}



export function AgentList({ agents, projectId }: AgentListProps) {

  if (agents.length === 0) {

    return <div className="empty-state">该项目下暂无 Agent，点击「新建 Agent」创建。</div>;

  }



  return (

    <div className="grid-cards">

      {agents.map((agent) => (

        <Link

          key={agent.id}

          href={`/projects/${projectId}/agents/${agent.id}`}

          className="card card-hover card-link"

        >

          <div className="flex items-start justify-between gap-3">

            <div className="min-w-0">

              <p className="font-semibold truncate">{agent.name}</p>

              <p className="text-muted mt-1.5">点击进入对话</p>

            </div>

            <span className={`badge shrink-0 ${statusBadge(agent.status)}`}>

              {agent.status}

            </span>

          </div>

        </Link>

      ))}

    </div>

  );

}

