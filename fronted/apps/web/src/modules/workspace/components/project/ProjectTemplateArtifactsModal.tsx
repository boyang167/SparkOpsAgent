"use client";

import { useState } from "react";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { Modal } from "@/components/ui/Modal";
import { ProjectYamlTab } from "./ProjectYamlTab";
import { ProjectJenkinsPreview } from "./ProjectJenkinsPreview";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";

type TemplateArtifactTab = "yaml" | "jenkins" | "dockerfile";

const INNER_TABS: { id: TemplateArtifactTab; label: string }[] = [
  { id: "yaml", label: "Yaml" },
  { id: "jenkins", label: "Jenkins" },
  { id: "dockerfile", label: "Dockerfile" },
];

interface ProjectTemplateArtifactsModalProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  project: ProjectVM;
}

export function ProjectTemplateArtifactsModal({
  open,
  onClose,
  workspaceId,
  project,
}: ProjectTemplateArtifactsModalProps) {
  const [innerTab, setInnerTab] = useState<TemplateArtifactTab>("yaml");

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Yaml / Jenkins / Dockerfile 配置模板"
      size="xl"
      panelClassName="project-template-artifacts-modal"
    >
      <div
        className="template-tabs-nav project-template-artifacts-inner-nav"
        role="tablist"
        aria-label="配置模板"
      >
        {INNER_TABS.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={innerTab === item.id}
            className={innerTab === item.id ? "is-active" : ""}
            onClick={() => setInnerTab(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>
      <div className="project-template-artifacts-inner-body">
        {innerTab === "yaml" ? (
          <ProjectYamlTab workspaceId={workspaceId} project={project} active={open} />
        ) : null}
        {innerTab === "jenkins" ? (
          <ProjectJenkinsPreview workspaceId={workspaceId} project={project} active={open} />
        ) : null}
        {innerTab === "dockerfile" ? (
          <section className="stack-sm">
            <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
          </section>
        ) : null}
      </div>
    </Modal>
  );
}
