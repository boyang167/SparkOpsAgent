"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { delivery } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { BuildVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";

interface DeliveryBuildPanelProps {
  workspaceId: string;
  projects: ProjectVM[];
  initialBuilds: BuildVM[];
  initialProjectId?: string;
}

function statusBadge(status: string) {
  if (status === "success") return "badge-success";
  if (status === "failed") return "badge-warning";
  if (status === "aborted") return "badge-neutral";
  if (status === "running") return "badge-neutral";
  return "badge-neutral";
}

export function DeliveryBuildPanel({
  workspaceId,
  projects,
  initialBuilds,
  initialProjectId,
}: DeliveryBuildPanelProps) {
  const router = useRouter();
  const [projectId, setProjectId] = useState(initialProjectId ?? projects[0]?.id ?? "");
  const [builds, setBuilds] = useState(initialBuilds);
  const [branch, setBranch] = useState("main");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const projectName = useMemo(
    () => projects.find((p) => p.id === projectId)?.name ?? "",
    [projects, projectId],
  );

  async function onProjectChange(nextId: string) {
    setProjectId(nextId);
    setError(null);
    if (!nextId) {
      setBuilds([]);
      return;
    }
    try {
      const items = await delivery.listBuilds(nextId, workspaceId);
      setBuilds(items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载构建记录失败");
    }
  }

  async function onTrigger() {
    if (!projectId) {
      setError("请先选择项目");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const created = await delivery.triggerBuild(projectId, workspaceId, branch);
      const items = await delivery.listBuilds(projectId, workspaceId);
      setBuilds(items);
      router.push(`/delivery/builds/${created.id}`);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "触发构建失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">Jenkins 构建</h2>
          <p className="text-muted mt-1">Mock 执行器模拟 PlatformSuite CI/CD 构建流程</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            className="input"
            value={projectId}
            onChange={(e) => void onProjectChange(e.target.value)}
          >
            {projects.length === 0 ? <option value="">无项目</option> : null}
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
          <input
            className="input w-28"
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder="分支"
          />
          <button className="btn" type="button" disabled={loading || !projectId} onClick={onTrigger}>
            {loading ? "构建中..." : "触发构建"}
          </button>
        </div>
      </div>

      {projectName ? (
        <p className="text-sm text-muted">
          当前项目：<span className="font-medium text-[var(--foreground)]">{projectName}</span>
        </p>
      ) : null}

      {error ? <p className="error">{error}</p> : null}

      {builds.length === 0 ? (
        <div className="empty-state">暂无构建记录</div>
      ) : (
        <div className="stack-md">
          {builds.map((build) => (
            <Link
              key={build.id}
              href={`/delivery/builds/${build.id}`}
              className="card block hover:border-[var(--border-strong)] transition-colors"
            >
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="font-medium text-sm">
                    #{build.build_number} · {build.branch}
                  </p>
                  <p className="text-xs text-muted mt-1">
                    {new Date(build.started_at).toLocaleString("zh-CN")}
                    {build.jenkins_job ? ` · ${build.jenkins_job}` : ""}
                  </p>
                </div>
                <span className={`badge shrink-0 ${statusBadge(build.status)}`}>{build.status}</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}
