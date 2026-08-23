"use client";

import { usePathname } from "next/navigation";
import { AdminNav } from "@/components/layout/AdminNav";
import { extractProjectId } from "@/components/layout/context-utils";
import { PlatformNav } from "@/components/layout/PlatformNav";
import { PlatformOpsNav } from "@/components/layout/PlatformOpsNav";
import { ProjectNav } from "@/components/layout/ProjectNav";
import { resolveConsoleKind } from "@/components/layout/nav-tree";

interface ProjectOption {
  id: string;
  name: string;
}

interface SidebarNavProps {
  workspaceId?: string;
  projects?: ProjectOption[];
}

export function SidebarNav({ workspaceId, projects }: SidebarNavProps) {
  const pathname = usePathname();
  const consoleKind = resolveConsoleKind(pathname);
  const projectId = extractProjectId(pathname);

  if (consoleKind === "admin") {
    return <AdminNav />;
  }

  if (consoleKind === "platform") {
    return <PlatformOpsNav />;
  }

  if (projectId) {
    const projectName = projects?.find((item) => item.id === projectId)?.name ?? "当前项目";
    return <ProjectNav projectId={projectId} projectName={projectName} />;
  }

  return <PlatformNav workspaceId={workspaceId} />;
}
