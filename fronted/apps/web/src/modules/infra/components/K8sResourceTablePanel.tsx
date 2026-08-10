"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface K8sResourceTablePanelProps {
  workspaceId: string;
  projects: ProjectVM[];
  kind: "configmaps" | "ingresses" | "services" | "pvcs" | "virtualservices";
  title: string;
  initialProjectId?: string;
}

export function K8sResourceTablePanel({
  workspaceId,
  projects,
  kind,
  title,
  initialProjectId,
}: K8sResourceTablePanelProps) {
  const [projectId, setProjectId] = useState(initialProjectId ?? projects[0]?.id ?? "");
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!projectId) {
      setItems([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const { infra } = await import("@sparkops/web-sdk");
      const data = await infra.listK8sResources(workspaceId, projectId, kind);
      setItems(data.items ?? []);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, projectId, kind]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <section className="section stack-md">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="section-title">{title}</h2>
        <select className="input" value={projectId} onChange={(e) => setProjectId(e.target.value)}>
          {projects.length === 0 ? <option value="">无项目</option> : null}
          {projects.map((project) => (
            <option key={project.id} value={project.id}>{project.name}</option>
          ))}
        </select>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无资源</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>命名空间</th>
                <th>详情</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row, index) => (
                <tr key={String(row.name ?? index)}>
                  <td className="font-mono">{String(row.name ?? "-")}</td>
                  <td>{String(row.namespace ?? "-")}</td>
                  <td className="text-xs text-muted font-mono truncate max-w-md">{JSON.stringify(row)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
