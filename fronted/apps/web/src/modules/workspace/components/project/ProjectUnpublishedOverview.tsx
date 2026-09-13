"use client";

import { useState, type ReactNode } from "react";
import type { ProjectPort, ProjectVM } from "@sparkops/contracts/workspace";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import { formatDateTime } from "./project-utils";

interface ProjectUnpublishedOverviewProps {
  project: ProjectVM;
  workspaceId: string;
  deploymentPage?: DeploymentRunListVM | null;
}

function portTags(ports: ProjectPort[] | undefined) {
  if (!ports?.length) return <span className="text-muted">-</span>;
  return (
    <span className="project-unpublished-tags">
      {ports.map((port, index) => (
        <span
          key={`${port.protocol ?? "TCP"}-${port.container_port}-${index}`}
          className="project-tag project-tag-gray"
        >
          {port.protocol ?? "TCP"}-{port.service_port ?? port.container_port}
        </span>
      ))}
    </span>
  );
}

function gitRefTypeLabel(value?: string) {
  if (value === "tag") return "tag";
  if (value === "commit") return "commit";
  return value || "branch";
}

function deploymentStatusLabel(status: string) {
  const map: Record<string, string> = {
    queued: "排队中",
    running: "运行中",
    succeeded: "成功",
    failed: "失败",
    cancelled: "已取消",
  };
  return map[status] ?? status;
}

function OverviewField({
  label,
  value,
  full,
}: {
  label: string;
  value: ReactNode;
  full?: boolean;
}) {
  return (
    <div className={full ? "is-full" : undefined}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function CopyableText({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  if (!value) return <span>-</span>;

  return (
    <span className="project-overview-copyable">
      <span className="project-overview-copyable-text">{value}</span>
      <button
        type="button"
        className="project-overview-copy-btn"
        onClick={() => {
          void navigator.clipboard.writeText(value).then(() => {
            setCopied(true);
            window.setTimeout(() => setCopied(false), 1500);
          });
        }}
      >
        {copied ? "已复制" : "复制"}
      </button>
    </span>
  );
}

export function ProjectUnpublishedOverview({
  project,
  workspaceId,
  deploymentPage = null,
}: ProjectUnpublishedOverviewProps) {
  const businessSpace = project.namespace || workspaceId;
  const replicas = project.resources?.replicas ?? 1;
  const memory = project.resources?.memory_limit || project.resources?.memory_request || "-";
  const cpu = project.resources?.cpu_limit || project.resources?.cpu_request || "-";
  const latestRun = deploymentPage?.items[0] ?? null;
  const approved = project.review_status === "approved";

  return (
    <div className="project-overview-page">
      {approved ? (
        <div className="project-overview-banner project-overview-banner-info project-overview-banner-soft">
          审核已通过。确认构建与运行配置无误后，点击右上角「发布」完成首次部署。
        </div>
      ) : null}

      {latestRun && latestRun.status !== "succeeded" ? (
        <div className="project-overview-banner project-overview-banner-warning project-overview-banner-soft">
          最近部署 {deploymentStatusLabel(latestRun.status)} · {latestRun.git_ref_value || "-"} ·{" "}
          {formatDateTime(latestRun.finished_at || latestRun.created_at)}
        </div>
      ) : null}

      <div className="project-overview-stats project-overview-stats-3">
        <div className="project-overview-stat">
          <span className="project-overview-stat-label">副本</span>
          <strong>{replicas}</strong>
        </div>
        <div className="project-overview-stat">
          <span className="project-overview-stat-label">CPU</span>
          <strong>{cpu}</strong>
        </div>
        <div className="project-overview-stat">
          <span className="project-overview-stat-label">内存</span>
          <strong>{memory}</strong>
        </div>
      </div>

      <div className="project-overview-layout">
        <section className="project-overview-panel">
          <h4 className="project-overview-panel-title">构建与源码</h4>
          <dl className="project-overview-fields">
            <OverviewField
              label="Git 地址"
              value={<CopyableText value={project.git_url || ""} />}
              full
            />
            <OverviewField label="分支类型" value={gitRefTypeLabel(project.git_ref_type)} />
            <OverviewField label="Git 版本" value={project.git_ref_value || "-"} />
            <OverviewField label="Dockerfile" value={project.dockerfile_path || "./Dockerfile"} full />
          </dl>
        </section>

        <section className="project-overview-panel">
          <h4 className="project-overview-panel-title">运行规格</h4>
          <dl className="project-overview-fields">
            <OverviewField
              label="命名空间"
              value={<span className="project-tag project-tag-blue">{businessSpace}</span>}
            />
            <OverviewField label="服务类型" value="集群内部访问" />
            <OverviewField
              label="容器规格"
              value={
                <span className="project-unpublished-tags">
                  <span className="project-tag project-tag-gray">{replicas} 副本</span>
                  <span className="project-tag project-tag-gray">CPU {cpu}</span>
                  <span className="project-tag project-tag-gray">内存 {memory}</span>
                </span>
              }
            />
            <OverviewField label="端口信息" value={portTags(project.resources?.ports)} full />
          </dl>
        </section>
      </div>
    </div>
  );
}
