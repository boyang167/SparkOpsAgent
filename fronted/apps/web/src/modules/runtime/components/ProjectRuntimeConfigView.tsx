"use client";

import { useEffect, useState, type ReactNode } from "react";
import { product, secret } from "@sparkops/web-sdk";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import { ProjectDockerfileViewer } from "@/modules/workspace/components/project/ProjectDockerfileViewer";
import {
  projectConfigActionLabel,
  shouldShowEditConfigAction,
} from "@/modules/workspace/components/project/project-progress";
import { countGeneratedTemplates } from "@/modules/workspace/components/project/project-template-render";

interface ProjectRuntimeConfigViewProps {
  workspaceId: string;
  project: ProjectVM;
  onConfigure: () => void;
}

function InfoGrid({
  items,
}: {
  items: Array<{ label: string; value: ReactNode; full?: boolean }>;
}) {
  return (
    <dl className="project-detail-grid">
      {items.map((item) => (
        <div key={item.label} className={`project-detail-cell${item.full ? " is-full" : ""}`}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function ProjectRuntimeConfigView({
  workspaceId,
  project,
  onConfigure,
}: ProjectRuntimeConfigViewProps) {
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [loading, setLoading] = useState(true);
  const canEdit = shouldShowEditConfigAction(project);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void Promise.all([
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
      product.listTemplates(workspaceId).catch(() => [] as TemplateVM[]),
    ])
      .then(([secretList, templateList]) => {
        if (cancelled) return;
        setSecrets(secretList);
        setTemplates(templateList);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, project.id]);

  const gitSecret = secrets.find((item) => item.id === project.secret_id);
  const registrySecret = secrets.find((item) => item.id === project.resources?.registry_secret_id);
  const jenkinsSecret = secrets.find((item) => item.id === project.jenkins_secret_id);
  const generatedTemplateCount = countGeneratedTemplates(project, templates);

  return (
    <div className="stack-md">
      {canEdit ? (
        <div className="flex justify-end">
          <button type="button" className="btn btn-secondary text-sm" onClick={onConfigure}>
            {projectConfigActionLabel(project)}
          </button>
        </div>
      ) : null}
      {loading ? <p className="text-muted">配置详情加载中…</p> : null}
      <InfoGrid
        items={[
          { label: "Git 仓库", value: project.git_url || "-", full: true },
          { label: "引用类型", value: project.git_ref_type || "branch" },
          { label: "引用值", value: project.git_ref_value || "-" },
          { label: "Dockerfile", value: project.dockerfile_path || "./Dockerfile" },
          {
            label: "Git 密钥",
            value: gitSecret ? gitSecret.display_name || gitSecret.name : project.secret_id || "未配置",
          },
          {
            label: "镜像仓库密钥",
            value: registrySecret
              ? registrySecret.display_name || registrySecret.name
              : project.resources?.registry_secret_id || "未配置",
          },
          {
            label: "Jenkins 密钥",
            value: jenkinsSecret
              ? jenkinsSecret.display_name || jenkinsSecret.name
              : project.jenkins_secret_id || "未配置",
          },
          { label: "默认集群", value: project.default_cluster_id || "未配置" },
          {
            label: "副本 / CPU / 内存",
            value: `${project.resources?.replicas ?? "-"} · ${project.resources?.cpu_request || "-"} ~ ${project.resources?.cpu_limit || "-"} · ${project.resources?.memory_request || "-"} ~ ${project.resources?.memory_limit || "-"}`,
            full: true,
          },
          {
            label: "生成模板数",
            value: generatedTemplateCount > 0 ? `${generatedTemplateCount} 个` : "暂无",
          },
        ]}
      />
      <section className="stack-sm">
        <h4 className="project-service-section-title">Dockerfile 预览</h4>
        <ProjectDockerfileViewer workspaceId={workspaceId} project={project} />
      </section>
    </div>
  );
}
