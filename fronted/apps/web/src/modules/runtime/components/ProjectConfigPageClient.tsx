"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectConfigurePanel } from "@/modules/workspace/components/project/ProjectConfigurePanel";
import { projectDisplayName } from "@/modules/workspace/components/project/project-utils";
import {
  projectConfigActionLabel,
  shouldShowEditConfigAction,
} from "@/modules/workspace/components/project/project-progress";
import { ProjectRuntimeConfigView } from "@/modules/runtime/components/ProjectRuntimeConfigView";

interface ProjectConfigPageClientProps {
  workspaceId: string;
  project: ProjectVM;
}

export function ProjectConfigPageClient({
  workspaceId,
  project: initialProject,
}: ProjectConfigPageClientProps) {
  const router = useRouter();
  const [project, setProject] = useState(initialProject);
  const [configureOpen, setConfigureOpen] = useState(false);

  useEffect(() => {
    setProject(initialProject);
  }, [initialProject]);

  return (
    <div className="stack-lg">
      <PageHeader
        title="配置"
        description={`${projectDisplayName(project)} 的代码源、运行资源与凭证。`}
        actions={
          shouldShowEditConfigAction(project) ? (
            <button type="button" className="btn-secondary" onClick={() => setConfigureOpen(true)}>
              {projectConfigActionLabel(project)}
            </button>
          ) : null
        }
      />
      <ProjectRuntimeConfigView workspaceId={workspaceId} project={project} />
      <ProjectConfigurePanel
        open={configureOpen}
        workspaceId={workspaceId}
        project={project}
        onClose={() => setConfigureOpen(false)}
        onProjectUpdated={(updated) => setProject(updated)}
        onSaved={() => {
          setConfigureOpen(false);
          router.refresh();
        }}
      />
    </div>
  );
}
