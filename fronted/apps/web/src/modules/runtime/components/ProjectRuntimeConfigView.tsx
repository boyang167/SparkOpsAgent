"use client";

import { useEffect, useState, type ReactNode } from "react";
import { platform, product, secret } from "@sparkops/web-sdk";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import { ProjectDockerfileViewer } from "@/modules/workspace/components/project/ProjectDockerfileViewer";
import { countGeneratedTemplates } from "@/modules/workspace/components/project/project-template-render";

interface ProjectRuntimeConfigViewProps {
  workspaceId: string;
  project: ProjectVM;
}

function secretLabel(item: SecretVM | undefined, fallbackId: string | undefined) {
  if (item) return item.display_name || item.name;
  return fallbackId || "";
}

function Field({ label, children, mono }: { label: string; children: ReactNode; mono?: boolean }) {
  return (
    <div className="pcfg-field">
      <dt>{label}</dt>
      <dd className={mono ? "is-mono" : undefined}>{children}</dd>
    </div>
  );
}

function CredentialRow({ label, value, loading }: { label: string; value: string; loading: boolean }) {
  const configured = Boolean(value);
  return (
    <li className="pcfg-cred">
      <span className="pcfg-cred-label">{label}</span>
      <span className={`pcfg-cred-value${configured ? "" : " is-missing"}`}>
        <span className="pcfg-cred-dot" aria-hidden="true" />
        {loading ? "…" : configured ? value : "未配置"}
      </span>
    </li>
  );
}

const REF_ICON: Record<string, string> = {
  branch: "M6 3v12M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 9a9 9 0 0 1-9 9",
  tag: "M3 12V3h9l9 9-9 9zM7.5 7.5h.01",
  commit: "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3 12h6M15 12h6",
};

function refTypeLabel(type?: string) {
  if (type === "tag") return "标签";
  if (type === "commit") return "提交";
  return "分支";
}

function parseRepo(url?: string) {
  const raw = url?.trim() ?? "";
  if (!raw) return { name: "", host: "" };
  const name = raw.replace(/\/+$/, "").split(/[/:]/).pop()?.replace(/\.git$/, "") ?? "";
  const host = raw.match(/^(?:[a-z+]+:\/\/)?(?:[^@/]+@)?([^/:]+)/i)?.[1] ?? "";
  return { name, host };
}

function range(min?: string, max?: string) {
  if (!min && !max) return "-";
  if (!max || min === max) return min || max || "-";
  if (!min) return max;
  return `${min} – ${max}`;
}

export function ProjectRuntimeConfigView({ workspaceId, project }: ProjectRuntimeConfigViewProps) {
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [clusters, setClusters] = useState<AuthorizedClusterVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void Promise.all([
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
      product.listTemplates(workspaceId).catch(() => [] as TemplateVM[]),
      platform.listAuthorizedWorkspaceClusters(workspaceId).catch(() => [] as AuthorizedClusterVM[]),
    ])
      .then(([secretList, templateList, clusterList]) => {
        if (cancelled) return;
        setSecrets(secretList);
        setTemplates(templateList);
        setClusters(clusterList);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, project.id]);

  const resources = project.resources;
  const gitSecret = secrets.find((item) => item.id === project.secret_id);
  const registrySecret = secrets.find((item) => item.id === resources?.registry_secret_id);
  const jenkinsSecret = secrets.find((item) => item.id === project.jenkins_secret_id);
  const generatedTemplateCount = countGeneratedTemplates(project, templates);
  const clusterId = project.default_cluster_id?.trim() ?? "";
  const cluster = clusters.find((item) => item.id === clusterId);
  const clusterLabel = cluster?.cluster_name || clusterId;
  const repo = parseRepo(project.git_url);

  async function copyGitUrl() {
    if (!project.git_url) return;
    try {
      await navigator.clipboard.writeText(project.git_url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable */
    }
  }

  return (
    <div className="pcfg">
      <section className="pcfg-card pcfg-source">
        <h2 className="pcfg-card-title">代码源</h2>
        <div className="pcfg-source-body">
          <div className="pcfg-repo">
            <span className="pcfg-repo-icon" aria-hidden="true">
              <Icon d="M6 3v12M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 9a9 9 0 0 1-9 9" />
            </span>
            <div className="pcfg-repo-copy">
              <span className="pcfg-repo-name">
                {repo.name || "未配置 Git 仓库"}
                {repo.host ? <span className="pcfg-repo-host">{repo.host}</span> : null}
              </span>
              {project.git_url ? (
                <span className="pcfg-repo-line">
                  <code className="pcfg-repo-url" title={project.git_url}>{project.git_url}</code>
                  <button
                    type="button"
                    className={`pcfg-icon-btn${copied ? " is-done" : ""}`}
                    onClick={() => void copyGitUrl()}
                    aria-label="复制仓库地址"
                    title={copied ? "已复制" : "复制仓库地址"}
                  >
                    {copied ? <Icon d="M5 12l5 5L20 7" /> : <Icon d="M9 9h10v10H9zM5 15V5h10" />}
                  </button>
                </span>
              ) : null}
            </div>
          </div>
          <dl className="pcfg-source-facts">
            <div className="pcfg-fact">
              <dt>{refTypeLabel(project.git_ref_type)}</dt>
              <dd>
                <Icon d={REF_ICON[project.git_ref_type || "branch"] ?? REF_ICON.branch} />
                {project.git_ref_value || "-"}
              </dd>
            </div>
            <div className="pcfg-fact">
              <dt>Dockerfile</dt>
              <dd className="is-mono">
                <Icon d="M14 3H6v18h12V7zM14 3v4h4" />
                {project.dockerfile_path || "./Dockerfile"}
              </dd>
            </div>
          </dl>
        </div>
      </section>

      <div className="pcfg-grid">
        <section className="pcfg-card">
          <h2 className="pcfg-card-title">运行资源</h2>
          <div className="pcfg-stats">
            <div className="pcfg-stat">
              <span className="pcfg-stat-value">{resources?.replicas ?? "-"}</span>
              <span className="pcfg-stat-label">副本</span>
            </div>
            <div className="pcfg-stat">
              <span className="pcfg-stat-value">{range(resources?.cpu_request, resources?.cpu_limit)}</span>
              <span className="pcfg-stat-label">CPU</span>
            </div>
            <div className="pcfg-stat">
              <span className="pcfg-stat-value">{range(resources?.memory_request, resources?.memory_limit)}</span>
              <span className="pcfg-stat-label">内存</span>
            </div>
          </div>
          <dl className="pcfg-fields">
            <Field label="默认集群">{clusterLabel || <span className="pcfg-muted">未配置</span>}</Field>
            <Field label="部署模板">
              {loading ? "…" : generatedTemplateCount > 0 ? `${generatedTemplateCount} 个` : <span className="pcfg-muted">暂无</span>}
            </Field>
          </dl>
        </section>

        <section className="pcfg-card">
          <h2 className="pcfg-card-title">凭证</h2>
          <ul className="pcfg-creds">
            <CredentialRow label="Git" value={secretLabel(gitSecret, project.secret_id)} loading={loading} />
            <CredentialRow
              label="镜像仓库"
              value={secretLabel(registrySecret, resources?.registry_secret_id)}
              loading={loading}
            />
            <CredentialRow label="Jenkins" value={secretLabel(jenkinsSecret, project.jenkins_secret_id)} loading={loading} />
          </ul>
        </section>
      </div>

      <section className="pcfg-dockerfile">
        <h2 className="pcfg-section-title">Dockerfile</h2>
        <ProjectDockerfileViewer workspaceId={workspaceId} project={project} lazy />
      </section>
    </div>
  );
}

function Icon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={d} />
    </svg>
  );
}
