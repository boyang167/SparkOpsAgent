"use client";

import { Modal } from "@/components/ui/Modal";
import { ProjectCreateWizard } from "./ProjectCreateWizard";
import type { ProjectVM } from "@sparkops/contracts/workspace";

interface ProjectCreateModalProps {
  open: boolean;
  workspaceId: string;
  onClose: () => void;
  onSuccess?: (project: ProjectVM) => void;
}

export function ProjectCreateModal({ open, workspaceId, onClose, onSuccess }: ProjectCreateModalProps) {
  return (
    <Modal open={open} onClose={onClose} title="创建项目" size="xl" panelClassName="project-create-modal">
      <ProjectCreateWizard
        open={open}
        workspaceId={workspaceId}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </Modal>
  );
}
