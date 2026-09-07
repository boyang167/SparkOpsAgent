"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  DeploymentOverviewVM,
  PodVM,
  ProjectRuntimeOverviewVM,
  ServiceOverviewVM,
} from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { PodDetailPanel, type PodDetailTab } from "@/modules/runtime/components/PodDetailPanel";
import { formatDateTime } from "./project-utils";
import { latestSuccessfulDeployment } from "./project-deployment-state";

interface ProjectPublishedOverviewProps {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
}

function phaseBadgeClass(phase: string) {
  if (phase === "Running" || phase === "Succeeded") return "badge-success";
  if (phase === "Pending") return "badge-warning";
  return "badge-warning";
}

function labelTags(labels: Record<string, string> | undefined) {
  if (!labels || Object.keys(labels).length === 0) {
    return <span>-</span>;
  }
  return (
    <span className="project-unpublished-tags">
      {Object.entries(labels).map(([key, value]) => (
        <span key={key} className="project-tag project-tag-gray">
          {key}:{value}
        </span>
      ))}
    </span>
  );
}

function endpointTags(endpoints: string[] | undefined) {
  if (!endpoints?.length) return <span>-</span>;
  return (
    <span className="project-unpublished-tags">
      {endpoints.map((endpoint) => (
        <span key={endpoint} className="project-tag project-tag-gray">
          {endpoint}
        </span>
      ))}
    </span>
  );
}

function servicePortTags(ports: ServiceOverviewVM["ports"]) {
  if (!ports?.length) return <span>-</span>;
  return (
    <span className="project-unpublished-tags">
      {ports.map((port, index) => (
        <span key={`${port.port}-${port.protocol}-${index}`} className="project-tag project-tag-gray">
          {port.port}-{port.protocol}
          {port.name ? `-${port.name}` : ""}
          {port.target_port ? `-${port.target_port}` : ""}
        </span>
      ))}
    </span>
  );
}

function RuntimeSectionCard({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className="project-runtime-card">
      <header className="project-runtime-card-header">
        <h4 className="project-runtime-card-title">{title}</h4>
        {action}
      </header>
      {children}
    </section>
  );
}

function PlaceholderRuntimeCard({ title }: { title: string }) {
  return (
    <RuntimeSectionCard
      title={title}
      action={
        <button type="button" className="project-runtime-card-add" disabled title="待开发">
          添加
        </button>
      }
    />
  );
}

function replicaStatus(deployment: DeploymentOverviewVM | undefined) {
  if (!deployment) return <span>-</span>;
  const available = deployment.available_replicas ?? 0;
  const total = deployment.replicas ?? 0;
  const failed = deployment.unavailable_replicas ?? 0;
  return (
    <span className="project-unpublished-tags">
      <span className="project-tag project-tag-green">
        {available} / {total} 个运行中
      </span>
      {failed > 0 ? <span className="project-tag project-tag-red">{failed} 个失败</span> : null}
    </span>
  );
}

function DetailItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="project-runtime-detail-item">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function MetricSparkline({ tone }: { tone: "cpu" | "memory" }) {
  return (
    <span className="project-runtime-metric">
      <span className={`project-runtime-sparkline project-runtime-sparkline-${tone}`} aria-hidden />
      <span className="project-runtime-metric-value">{tone === "memory" ? "0 MB" : "0"}</span>
    </span>
  );
}

export function ProjectPublishedOverview({
  workspaceId,
  project,
  deploymentPage,
}: ProjectPublishedOverviewProps) {
  const [overview, setOverview] = useState<ProjectRuntimeOverviewVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [podPanel, setPodPanel] = useState<{ pod: PodVM; tab: PodDetailTab } | null>(null);
  const [restartingPodId, setRestartingPodId] = useState<string | null>(null);

  const latestDeploy = latestSuccessfulDeployment(deploymentPage);
  const pods = overview?.pods ?? [];
  const deployment = overview?.deployment;
  const service = overview?.service;
  const namespace = deployment?.namespace || project.namespace || workspaceId;
  const imageTag = deployment?.image_tag || latestDeploy?.image_tag;
  const memoryLimit = deployment?.memory_limit || project.resources?.memory_limit;

  const loadOverview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await cluster.getProjectRuntimeOverview(project.id, workspaceId);
      setOverview(data);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载运行概览失败");
      setOverview(null);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  const handleRestartPod = useCallback(
    async (pod: PodVM) => {
      if (!window.confirm(`确定重启 Pod「${pod.name}」吗？`)) {
        return;
      }
      setRestartingPodId(pod.id);
      setError(null);
      try {
        await cluster.restartPod(project.id, pod.id, workspaceId);
        await loadOverview();
      } catch (err) {
        setError(err instanceof BffError ? err.message : "重启 Pod 失败");
      } finally {
        setRestartingPodId(null);
      }
    },
    [loadOverview, project.id, workspaceId],
  );

  return (
    <div className="project-runtime-overview">
      {error ? <p className="error">{error}</p> : null}

      <section className="project-runtime-card">
        <header className="project-runtime-card-header">
          <h4 className="project-runtime-card-title">详情</h4>
          <button
            type="button"
            className="project-runtime-card-action"
            onClick={() => void loadOverview()}
            disabled={loading}
            title="刷新"
          >
            ↻
          </button>
        </header>

        <dl className="project-runtime-detail-grid">
          <DetailItem label="名称" value={deployment?.name || project.name} />
          <DetailItem label="命名空间" value={namespace} />
          <DetailItem label="标签" value={loading ? "加载中…" : labelTags(deployment?.labels)} />

          <DetailItem
            label="当前镜像版本"
            value={
              imageTag ? <span className="project-tag project-tag-blue">{imageTag}</span> : "-"
            }
          />
          <DetailItem label="升级策略" value={deployment?.strategy || "RollingUpdate"} />
          <DetailItem
            label="TOKEN"
            value={
              deployment?.uid ? (
                <span className="project-tag project-tag-orange">{deployment.uid}</span>
              ) : (
                "-"
              )
            }
          />

          <DetailItem
            label="创建时间"
            value={
              deployment?.created_at
                ? formatDateTime(deployment.created_at)
                : latestDeploy?.finished_at
                  ? formatDateTime(latestDeploy.finished_at)
                  : "-"
            }
          />
          <DetailItem
            label="配置资源"
            value={
              memoryLimit ? (
                <span className="project-tag project-tag-blue">最大内存: {memoryLimit}</span>
              ) : (
                "-"
              )
            }
          />
          <DetailItem
            label="Git"
            value={
              project.git_url ? <span className="project-tag project-tag-gray">{project.git_url}</span> : "-"
            }
          />

          <DetailItem
            label="镜像"
            value={deployment?.image || (imageTag ? `${project.name}:${imageTag}` : "-")}
          />
          <DetailItem label="容器组" value={loading ? "加载中…" : replicaStatus(deployment)} />
          <DetailItem label="命令、参数" value="-" />

          <DetailItem label="hosts" value="-" />
        </dl>
      </section>

      <section className="project-runtime-card">
        <header className="project-runtime-card-header">
          <h4 className="project-runtime-card-title">容器组</h4>
        </header>

        {!loading && pods.length === 0 ? (
          <p className="project-runtime-empty">暂无 Pod，请确认 Deployment 已成功调度。</p>
        ) : (
          <div className="project-runtime-table-wrap">
            <table className="project-runtime-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>节点</th>
                  <th>状态</th>
                  <th>已重启</th>
                  <th>CPU（核）</th>
                  <th>内存（字节）</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {pods.map((pod) => (
                  <tr key={pod.id}>
                    <td>
                      <button
                        type="button"
                        className="project-runtime-pod-link"
                        onClick={() => setPodPanel({ pod, tab: "overview" })}
                      >
                        {pod.name}
                      </button>
                      {pod.message ? (
                        <div className="project-runtime-pod-message">{pod.message}</div>
                      ) : null}
                    </td>
                    <td>{pod.node || "-"}</td>
                    <td>
                      <span className={`badge ${phaseBadgeClass(pod.phase)}`}>{pod.phase}</span>
                    </td>
                    <td>{pod.restarts}</td>
                    <td>
                      <MetricSparkline tone="cpu" />
                    </td>
                    <td>
                      <MetricSparkline tone="memory" />
                    </td>
                    <td>{pod.started_at ? formatDateTime(pod.started_at) : "-"}</td>
                    <td>
                      <div className="project-runtime-row-actions">
                        <button
                          type="button"
                          className="project-runtime-action"
                          disabled={loading || restartingPodId === pod.id}
                          onClick={() => void handleRestartPod(pod)}
                        >
                          {restartingPodId === pod.id ? "重启中…" : "重启"}
                        </button>
                        <span className="project-runtime-action-sep">|</span>
                        <button
                          type="button"
                          className="project-runtime-action"
                          onClick={() => setPodPanel({ pod, tab: "logs" })}
                        >
                          日志
                        </button>
                        <span className="project-runtime-action-sep">|</span>
                        <button
                          type="button"
                          className="project-runtime-action"
                          onClick={() => setPodPanel({ pod, tab: "console" })}
                        >
                          控制台
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <PlaceholderRuntimeCard title="日志采集" />
      <PlaceholderRuntimeCard title="健康检测" />

      {!loading && service ? (
        <RuntimeSectionCard
          title="Service"
          action={
            <div className="project-runtime-card-actions">
              <button type="button" className="project-runtime-card-add" disabled title="待开发">
                调整端口
              </button>
            </div>
          }
        >
          <dl className="project-runtime-detail-grid">
            <DetailItem label="名称" value={service.name} />
            <DetailItem label="标签" value={labelTags(service.labels)} />
            <DetailItem label="选择器" value={labelTags(service.selector)} />
            <DetailItem label="端口" value={servicePortTags(service.ports)} />
            <DetailItem label="注释" value={labelTags(service.annotations)} />
            <DetailItem label="端点" value={endpointTags(service.endpoints)} />
            <DetailItem
              label="创建时间"
              value={service.created_at ? formatDateTime(service.created_at) : "-"}
            />
          </dl>
        </RuntimeSectionCard>
      ) : (
        <RuntimeSectionCard
          title="Service"
          action={
            <button type="button" className="project-runtime-card-add" disabled title="待开发">
              创建服务
            </button>
          }
        >
          {!loading ? <p className="project-runtime-empty">暂无 Service。</p> : null}
        </RuntimeSectionCard>
      )}

      <RuntimeSectionCard
        title="外部地址"
        action={
          <button type="button" className="project-runtime-card-add" disabled title="待开发">
            添加
          </button>
        }
      />

      {podPanel ? (
        <PodDetailPanel
          workspaceId={workspaceId}
          projectId={project.id}
          pod={podPanel.pod}
          initialTab={podPanel.tab}
          onClose={() => setPodPanel(null)}
        />
      ) : null}
    </div>
  );
}
