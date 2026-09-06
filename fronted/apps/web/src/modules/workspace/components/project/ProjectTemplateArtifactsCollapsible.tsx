"use client";

import { useState } from "react";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { ProjectYamlTab } from "./ProjectYamlTab";
import { ProjectJenkinsPreview } from "./ProjectJenkinsPreview";
import { ProjectDockerfileViewer } from "./ProjectDockerfileViewer";

type TemplateArtifactTab = "yaml" | "jenkins" | "dockerfile";

const INNER_TABS: { id: TemplateArtifactTab; label: string }[] = [
  { id: "yaml", label: "Yaml" },
  { id: "jenkins", label: "Jenkins" },
  { id: "dockerfile", label: "Dockerfile" },
];

interface ProjectTemplateArtifactsCollapsibleProps {
  workspaceId: string;
  project: ProjectVM;
}

export function ProjectTemplateArtifactsCollapsible({
  workspaceId,
  project,
}: ProjectTemplateArtifactsCollapsibleProps) {
  const [expanded, setExpanded] = useState(false);
  const [innerTab, setInnerTab] = useState<TemplateArtifactTab>("yaml");

  return (
    <section className="project-template-artifacts-collapsible">
      <button
        type="button"
        className="project-template-artifacts-toggle"
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <span className="project-template-artifacts-toggle-label">Yaml / Jenkins / Dockerfile 配置模板</span>
        <span className="project-template-artifacts-toggle-action">{expanded ? "收起" : "展开"}</span>
      </button>
      {expanded ? (
        <div className="project-template-artifacts-body">
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
              <ProjectYamlTab workspaceId={workspaceId} project={project} active={expanded} />
            ) : null}
            {innerTab === "jenkins" ? (
              <ProjectJenkinsPreview workspaceId={workspaceId} project={project} active={expanded} />
            ) : null}
            {innerTab === "dockerfile" ? (
              <section className="stack-sm">
                <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
              </section>
            ) : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
