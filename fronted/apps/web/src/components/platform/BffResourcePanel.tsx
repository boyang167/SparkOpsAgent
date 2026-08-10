"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { bffFetch } from "@sparkops/web-sdk";

interface BffResourcePanelProps {
  workspaceId: string;
  bffPath: string;
  upstreamPath: string;
  title?: string;
  projects?: ProjectVM[];
  requiresProject?: boolean;
  initialProjectId?: string;
}

function formatJson(value: unknown) {
  return JSON.stringify(value, null, 2);
}

export function BffResourcePanel({
  workspaceId,
  bffPath,
  upstreamPath,
  title = "数据",
  projects = [],
  requiresProject = false,
  initialProjectId,
}: BffResourcePanelProps) {
  const [projectId, setProjectId] = useState(
    initialProjectId ?? projects[0]?.id ?? "",
  );
  const [data, setData] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const resolvedBffPath = bffPath
    .replace("{wsId}", workspaceId)
    .replace("{projectId}", projectId);

  const load = useCallback(async () => {
    if (requiresProject && !projectId) {
      setData(null);
      setError(null);
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const result = await bffFetch<unknown>(resolvedBffPath, { workspaceId });
      setData(result);
    } catch (err) {
      setData(null);
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [requiresProject, projectId, resolvedBffPath, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">{title}</h2>
          <p className="text-muted mt-1 text-sm">
            BFF: <code className="text-xs">/api/bff/v1{bffPath.replace("{wsId}", workspaceId).replace("{projectId}", projectId || ":projectId")}</code>
          </p>
          <p className="text-muted mt-1 text-sm">
            Upstream: <code className="text-xs">{upstreamPath.replace("{projectId}", projectId || ":projectId")}</code>
          </p>
        </div>
        {requiresProject ? (
          <select
            className="input"
            value={projectId}
            onChange={(e) => setProjectId(e.target.value)}
          >
            {projects.length === 0 ? <option value="">无项目</option> : null}
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        ) : (
          <button type="button" className="btn btn-secondary" onClick={() => void load()}>
            刷新
          </button>
        )}
      </div>

      {loading ? (
        <p className="text-muted text-sm">加载中…</p>
      ) : error ? (
        <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 stack-sm">
          <p className="text-sm text-[var(--warning)]">{error}</p>
          <p className="text-muted text-sm">
            后端 API 尚未就绪或暂无数据。预期上游路径：<code>{upstreamPath}</code>
          </p>
        </div>
      ) : data ? (
        <pre className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4 text-xs overflow-auto max-h-[480px]">
          {formatJson(data)}
        </pre>
      ) : (
        <p className="text-muted text-sm">暂无数据</p>
      )}
    </section>
  );
}
