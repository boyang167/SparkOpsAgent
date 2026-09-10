"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectConfigurePanel } from "@/modules/workspace/components/project/ProjectConfigurePanel";
import { projectDisplayName } from "@/modules/workspace/components/project/project-utils";
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
        eyebrow={projectDisplayName(project)}
        title="配置"
        description="管理项目运行配置（含 K8s ConfigMap）、密钥与构建相关参数。"
      />
      <ProjectRuntimeConfigView
        workspaceId={workspaceId}
        project={project}
        onConfigure={() => setConfigureOpen(true)}
      />
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
