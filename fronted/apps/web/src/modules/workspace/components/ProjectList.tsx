import Link from "next/link";

import type { ProjectVM } from "@sparkops/contracts/workspace";



interface ProjectListProps {

  projects: ProjectVM[];

  workspaceId: string;

}



function statusBadge(status: string) {

  if (status === "active") {

    return "badge-success";

  }

  return "badge-neutral";

}



export function ProjectList({ projects }: ProjectListProps) {

  if (projects.length === 0) {

    return <div className="empty-state">当前 Workspace 下暂无项目，点击「新建项目」开始。</div>;

  }



  return (

    <div className="grid-cards">

      {projects.map((project) => (

        <Link

          key={project.id}

          href={`/projects/${project.id}`}

          className="card card-hover card-link"

        >

          <div className="flex items-start justify-between gap-3">

            <div className="min-w-0">

              <p className="font-semibold truncate">{project.name}</p>

              {project.description ? (

                <p className="text-muted mt-1.5 line-clamp-2">{project.description}</p>

              ) : (

                <p className="text-muted mt-1.5">进入项目查看 Agent</p>

              )}

            </div>

            <span className={`badge shrink-0 ${statusBadge(project.status)}`}>

              {project.status}

            </span>

          </div>

        </Link>

      ))}

    </div>

  );

}

