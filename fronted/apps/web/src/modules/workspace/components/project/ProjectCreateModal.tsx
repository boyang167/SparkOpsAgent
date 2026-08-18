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
    <Modal
      open={open}
      onClose={onClose}
      title="创建项目"
      description="请仔细核对每一项内容和参数。"
      size="xl"
      panelClassName="project-create-modal"
    >
      <ProjectCreateWizard
        open={open}
        workspaceId={workspaceId}
        onClose={onClose}
        onSuccess={onSuccess}
      />
    </Modal>
  );
}
