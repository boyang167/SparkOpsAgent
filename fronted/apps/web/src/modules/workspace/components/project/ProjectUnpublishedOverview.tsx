"use client";

import type { ProjectPort, ProjectVM } from "@sparkops/contracts/workspace";
import { formatDateTime, languageLabel, projectDisplayName } from "./project-utils";

interface ProjectUnpublishedOverviewProps {
  project: ProjectVM;
  workspaceId: string;
}

function portTags(ports: ProjectPort[] | undefined) {
  if (!ports?.length) return <span>-</span>;
  return (
    <span className="project-unpublished-tags">
      {ports.map((port, index) => (
        <span key={`${port.protocol ?? "TCP"}-${port.container_port}-${index}`} className="project-tag project-tag-gray">
          {port.protocol ?? "TCP"}-{port.service_port ?? port.container_port}
        </span>
      ))}
    </span>
  );
}

function memoryLabel(project: ProjectVM) {
  const memory = project.resources?.memory_limit || project.resources?.memory_request;
  return memory ? `${memory} 内存` : "-";
}

function gitRefTypeLabel(value?: string) {
  if (value === "tag") return "tag";
  if (value === "commit") return "commit";
  return value || "branch";
}

export function ProjectUnpublishedOverview({ project, workspaceId }: ProjectUnpublishedOverviewProps) {
  const businessSpace = project.namespace || workspaceId;
  const replicas = project.resources?.replicas ?? 1;

  return (
    <section className="project-unpublished-card">
      <h4 className="project-unpublished-card-title">项目信息</h4>

      <dl className="project-unpublished-grid">
        <div>
          <dt>申请人</dt>
          <dd>{project.submitted_by || project.reviewed_by || "-"}</dd>
        </div>
        <div>
          <dt>业务空间</dt>
          <dd>{businessSpace}</dd>
        </div>
        <div>
          <dt>英文名称</dt>
          <dd>{project.name}</dd>
        </div>
        <div>
          <dt>中文名称</dt>
          <dd>{projectDisplayName(project)}</dd>
        </div>
        <div>
          <dt>项目语言</dt>
          <dd>{languageLabel(project.language)}</dd>
        </div>
        <div>
          <dt>创建时间</dt>
          <dd>{project.created_at ? formatDateTime(project.created_at) : "-"}</dd>
        </div>
      </dl>

      <h5 className="project-unpublished-subtitle">详细信息</h5>
      <dl className="project-unpublished-grid">
        <div>
          <dt>Git地址</dt>
          <dd>{project.git_url || "-"}</dd>
        </div>
        <div>
          <dt>分支类型</dt>
          <dd>{gitRefTypeLabel(project.git_ref_type)}</dd>
        </div>
        <div>
          <dt>Git版本号</dt>
          <dd>{project.git_ref_value || "-"}</dd>
        </div>
        <div>
          <dt>Docker地址</dt>
          <dd>{project.dockerfile_path || "./Dockerfile"}</dd>
        </div>
        <div>
          <dt>容器数量及规格</dt>
          <dd className="project-unpublished-tags">
            <span className="project-tag project-tag-gray">{replicas} 个</span>
            <span className="project-tag project-tag-gray">{memoryLabel(project)}</span>
          </dd>
        </div>
        <div>
          <dt>服务类型</dt>
          <dd>集群内部访问</dd>
        </div>
        <div>
          <dt>端口信息</dt>
          <dd>{portTags(project.resources?.ports)}</dd>
        </div>
      </dl>
    </section>
  );
}
