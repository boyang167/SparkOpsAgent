"use client";

import { useState } from "react";
import { ProjectCreateModal } from "./project/ProjectCreateModal";

interface CreateProjectFormProps {
  workspaceId: string;
}

export function CreateProjectForm({ workspaceId }: CreateProjectFormProps) {
  const [open, setOpen] = useState(false);

  if (!workspaceId) return null;

  return (
    <>
      <button type="button" className="btn" onClick={() => setOpen(true)}>
        创建项目
      </button>
      <ProjectCreateModal open={open} workspaceId={workspaceId} onClose={() => setOpen(false)} />
    </>
  );
}
