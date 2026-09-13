"use client";

import { useMemo, useState } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PodVM } from "@sparkops/contracts/cluster";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { K8sEmptyState } from "@/components/k8s";
import { PodDetailPanel } from "./PodDetailPanel";

interface RuntimePodPanelProps {
  workspaceId: string;
  projects: ProjectVM[];
  initialPods: PodVM[];
  initialProjectId?: string;
}

function phaseBadge(phase: string) {
  if (phase === "Running") return "badge-success";
  if (phase === "Pending") return "badge-neutral";
  return "badge-warning";
}

export function RuntimePodPanel({
  workspaceId,
  projects,
  initialPods,
  initialProjectId,
}: RuntimePodPanelProps) {
  const [projectId, setProjectId] = useState(initialProjectId ?? projects[0]?.id ?? "");
  const [pods, setPods] = useState(initialPods);
  const [error, setError] = useState<string | null>(null);
  const [selectedPod, setSelectedPod] = useState<PodVM | null>(null);

  const projectName = useMemo(
    () => projects.find((p) => p.id === projectId)?.name ?? "",
    [projects, projectId],
  );

  async function onProjectChange(nextId: string) {
    setProjectId(nextId);
    setError(null);
    setSelectedPod(null);
    if (!nextId) {
      setPods([]);
      return;
    }
    try {
      const items = await cluster.listPods(nextId, workspaceId);
      setPods(items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载 Pod 失败");
    }
  }

  return (
    <section className="section stack-md">
      <div className="section-header !mb-0">
        <div>
          <h2 className="section-title">Pod 列表</h2>
          <p className="text-muted mt-1">Mock K8s 数据，对应 PlatformSuite 应用运维视图</p>
        </div>
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
      </div>

      {projectName ? (
        <p className="text-sm text-muted">
          当前项目：<span className="font-medium text-[var(--foreground)]">{projectName}</span>
        </p>
      ) : null}

      {error ? <p className="error">{error}</p> : null}

      {pods.length === 0 ? (
        <K8sEmptyState variant="empty" title="暂无 Pod" compact />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-muted border-b border-[var(--border)]">
                <th className="py-2 pr-4">名称</th>
                <th className="py-2 pr-4">命名空间</th>
                <th className="py-2 pr-4">状态</th>
                <th className="py-2 pr-4">节点</th>
                <th className="py-2 pr-4">镜像</th>
                <th className="py-2">重启</th>
              </tr>
            </thead>
            <tbody>
              {pods.map((pod) => (
                <tr
                  key={pod.id}
                  className={`border-b border-[var(--border)] cursor-pointer hover:bg-[var(--surface)] transition-colors ${
                    selectedPod?.id === pod.id ? "bg-[var(--surface)]" : ""
                  }`}
                  onClick={() => setSelectedPod(pod)}
                >
                  <td className="py-3 pr-4 font-medium">{pod.name}</td>
                  <td className="py-3 pr-4">{pod.namespace}</td>
                  <td className="py-3 pr-4">
                    <span className={`badge ${phaseBadge(pod.phase)}`}>{pod.phase}</span>
                  </td>
                  <td className="py-3 pr-4">{pod.node}</td>
                  <td className="py-3 pr-4 font-mono text-xs">{pod.image}</td>
                  <td className="py-3">{pod.restarts}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedPod && projectId ? (
        <PodDetailPanel
          workspaceId={workspaceId}
          projectId={projectId}
          pod={selectedPod}
          onClose={() => setSelectedPod(null)}
        />
      ) : null}
    </section>
  );
}
