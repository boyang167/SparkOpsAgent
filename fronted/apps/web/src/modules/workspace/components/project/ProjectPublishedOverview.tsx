"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PodVM } from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { PodDetailPanel } from "@/modules/runtime/components/PodDetailPanel";
import { formatDateTime } from "./project-utils";
import { latestSuccessfulDeployment } from "./project-deployment-state";

interface ProjectPublishedOverviewProps {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
}

function phaseBadgeClass(phase: string) {
  if (phase === "Running") return "badge-success";
  if (phase === "Pending") return "badge-neutral";
  return "badge-warning";
}

export function ProjectPublishedOverview({
  workspaceId,
  project,
  deploymentPage,
}: ProjectPublishedOverviewProps) {
  const [pods, setPods] = useState<PodVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPod, setSelectedPod] = useState<PodVM | null>(null);

  const latestDeploy = latestSuccessfulDeployment(deploymentPage);

  const loadPods = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await cluster.listPods(project.id, workspaceId);
      setPods(items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载 Pod 失败");
      setPods([]);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    void loadPods();
  }, [loadPods]);

  return (
    <div className="stack-md">
      <section className="stack-sm">
        <div className="flex items-center justify-between gap-3">
          <h4 className="project-service-section-title">运行概览</h4>
          <button
            type="button"
            className="btn btn-secondary text-sm"
            onClick={() => void loadPods()}
            disabled={loading}
          >
            {loading ? "刷新中…" : "刷新"}
          </button>
        </div>

        <InfoGrid
          items={[
            { label: "项目", value: project.name },
            { label: "命名空间", value: project.namespace || workspaceId },
            {
              label: "最近成功发布",
              value: latestDeploy
                ? `${latestDeploy.git_ref_value} · ${latestDeploy.image_tag || "-"}`
                : "-",
            },
            {
              label: "发布时间",
              value: latestDeploy?.finished_at
                ? formatDateTime(latestDeploy.finished_at)
                : latestDeploy?.created_at
                  ? formatDateTime(latestDeploy.created_at)
                  : "-",
            },
            { label: "Pod 数量", value: loading ? "加载中…" : String(pods.length) },
            {
              label: "运行中",
              value: loading
                ? "加载中…"
                : String(pods.filter((pod) => pod.phase === "Running").length),
            },
          ]}
        />
      </section>

      <section className="stack-sm">
        <h4 className="project-service-section-title">Pod 列表</h4>
        {error ? <p className="error">{error}</p> : null}
        {!error && !loading && pods.length === 0 ? (
          <div className="project-service-placeholder">
            <p>暂无 Pod，请确认集群中已部署该服务。</p>
          </div>
        ) : null}
        {pods.length > 0 ? (
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
                      <span className={`badge ${phaseBadgeClass(pod.phase)}`}>{pod.phase}</span>
                    </td>
                    <td className="py-3 pr-4">{pod.node || "-"}</td>
                    <td className="py-3 pr-4 font-mono text-xs">{pod.image}</td>
                    <td className="py-3">{pod.restarts}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      {selectedPod ? (
        <PodDetailPanel
          workspaceId={workspaceId}
          projectId={project.id}
          pod={selectedPod}
          onClose={() => setSelectedPod(null)}
        />
      ) : null}
    </div>
  );
}

function InfoGrid({
  items,
}: {
  items: Array<{ label: string; value: ReactNode }>;
}) {
  return (
    <dl className="project-detail-grid">
      {items.map((item) => (
        <div key={item.label} className="project-detail-cell">
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
